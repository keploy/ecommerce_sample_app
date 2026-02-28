# Kafka Compression Support Bugfix Design

## Overview

The `decodeRecordBatches` function currently only implements decompression for gzip-compressed Kafka record batches. While the code correctly identifies all five compression types (none, gzip, snappy, lz4, zstd), it only decompresses gzip. This causes record batches compressed with snappy, lz4, or zstd to be decoded as raw compressed bytes, resulting in corrupted data. The fix will add decompression support for the three missing compression algorithms using minimal code changes to preserve existing functionality.

## Glossary

- **Bug_Condition (C)**: The condition that triggers the bug - when record batches use snappy (2), lz4 (3), or zstd (4) compression
- **Property (P)**: The desired behavior when compressed batches are received - records should be decompressed before decoding
- **Preservation**: Existing decompression behavior for gzip and no-compression handling that must remain unchanged
- **decodeRecordBatches**: The function in `pkg/core/proxy/integrations/kafka/wire/bodies.go` that parses Kafka record batch wire format
- **compression**: An int value (0-4) extracted from the attributes field indicating the compression algorithm used
- **recordsData**: The byte slice containing either compressed or uncompressed record data that needs decoding

## Bug Details

### Fault Condition

The bug manifests when Kafka producers send record batches compressed with snappy, lz4, or zstd algorithms. The `decodeRecordBatches` function correctly identifies the compression type from the attributes field but only implements decompression for gzip (type 1), leaving the other three compression types unhandled.

**Formal Specification:**
```
FUNCTION isBugCondition(input)
  INPUT: input of type RecordBatchBytes
  OUTPUT: boolean
  
  RETURN input.compressionType IN [2, 3, 4]
         AND input.recordsData.isCompressed == true
         AND decompression is not performed
END FUNCTION
```

### Examples

- **Snappy compression**: A producer sends a batch with compression=2. The decoder sets `batch.Compression = "snappy"` but passes compressed bytes directly to `decodeRecords()`, which fails to parse them correctly.
- **LZ4 compression**: A producer sends a batch with compression=3. The decoder sets `batch.Compression = "lz4"` but the compressed recordsData is never decompressed, resulting in garbage record data.
- **Zstd compression**: A producer sends a batch with compression=4. The decoder sets `batch.Compression = "zstd"` but attempts to decode compressed bytes as raw records, producing invalid output.
- **Gzip compression (working)**: A producer sends a batch with compression=1. The decoder correctly decompresses using gzip before calling `decodeRecords()`.

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- Uncompressed batches (compression=0) must continue to decode without any decompression step
- Gzip-compressed batches (compression=1) must continue to decompress correctly using the existing gzip logic
- Error handling for gzip decompression failures must remain unchanged
- All RecordBatch field parsing (BaseOffset, BatchLength, timestamps, producer info, etc.) must remain unchanged

**Scope:**
All inputs that do NOT involve snappy, lz4, or zstd compression should be completely unaffected by this fix. This includes:
- Uncompressed record batches (compression=0)
- Gzip-compressed record batches (compression=1)
- Batch header parsing logic
- Record decoding logic after decompression

## Hypothesized Root Cause

Based on the bug description and code analysis, the root cause is clear:

1. **Incomplete Implementation**: The function was implemented with only gzip decompression support, likely because:
   - Gzip is the most common compression type in older Kafka deployments
   - The other compression libraries weren't imported or integrated initially
   - The feature was partially implemented and never completed

2. **Missing Library Integration**: The code needs to import and use three additional decompression libraries:
   - `github.com/golang/snappy` for snappy decompression
   - `github.com/pierrec/lz4/v4` for lz4 decompression
   - `github.com/klauspost/compress/zstd` for zstd decompression

3. **Switch Statement Gap**: The switch statement on line 2447-2458 identifies compression types but the decompression logic (lines 2517-2527) only handles gzip (compression == 1).

## Correctness Properties

Property 1: Fault Condition - Snappy/LZ4/Zstd Decompression

_For any_ record batch where the compression type is 2 (snappy), 3 (lz4), or 4 (zstd), the fixed decodeRecordBatches function SHALL decompress the recordsData using the appropriate decompression algorithm before passing it to decodeRecords, resulting in correctly parsed record data.

**Validates: Requirements 2.1, 2.2, 2.3, 2.4**

Property 2: Preservation - Existing Compression Handling

_For any_ record batch where the compression type is 0 (none) or 1 (gzip), the fixed decodeRecordBatches function SHALL produce exactly the same behavior as the original function, preserving the existing decompression logic and error handling.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4**

## Fix Implementation

### Changes Required

Assuming our root cause analysis is correct:

**File**: `pkg/core/proxy/integrations/kafka/wire/bodies.go`

**Function**: `decodeRecordBatches`

**Specific Changes**:
1. **Add Import Statements**: Add three new imports at the top of the file:
   - `"github.com/golang/snappy"`
   - `"github.com/pierrec/lz4/v4"`
   - `"github.com/klauspost/compress/zstd"`

2. **Extend Decompression Logic**: Modify the decompression section (currently lines 2517-2527) to handle all compression types:
   - Add case for compression == 2 (snappy): Use `snappy.Decode()` to decompress
   - Add case for compression == 3 (lz4): Use `lz4.NewReader()` and `io.ReadAll()` to decompress
   - Add case for compression == 4 (zstd): Use `zstd.NewReader()` and `io.ReadAll()` to decompress
   - Keep existing gzip logic unchanged

3. **Error Handling**: Follow the same error handling pattern as gzip:
   - If decompression fails, silently continue with compressed data (matches current gzip behavior)
   - This preserves the existing graceful degradation approach

4. **Minimal Code Changes**: Use a switch statement or if-else chain to keep changes localized to the decompression section only

## Testing Strategy

### Validation Approach

The testing strategy follows a two-phase approach: first, surface counterexamples that demonstrate the bug on unfixed code, then verify the fix works correctly and preserves existing behavior.

### Exploratory Fault Condition Checking

**Goal**: Surface counterexamples that demonstrate the bug BEFORE implementing the fix. Confirm that snappy, lz4, and zstd compressed batches fail to decompress correctly.

**Test Plan**: Create test record batches with each compression type, compress sample record data using each algorithm, and attempt to decode them with the UNFIXED code. Observe that snappy/lz4/zstd batches produce incorrect record data while gzip works correctly.

**Test Cases**:
1. **Snappy Decompression Test**: Create a batch with compression=2 and snappy-compressed records (will fail on unfixed code)
2. **LZ4 Decompression Test**: Create a batch with compression=3 and lz4-compressed records (will fail on unfixed code)
3. **Zstd Decompression Test**: Create a batch with compression=4 and zstd-compressed records (will fail on unfixed code)
4. **Gzip Decompression Test**: Create a batch with compression=1 and gzip-compressed records (should pass on unfixed code)

**Expected Counterexamples**:
- Snappy/lz4/zstd batches will produce empty or corrupted record arrays
- The `decodeRecords()` function will fail to parse compressed bytes as valid record structures
- Possible symptoms: zero records decoded, panic from invalid varint encoding, or garbage data

### Fix Checking

**Goal**: Verify that for all inputs where the bug condition holds, the fixed function produces the expected behavior.

**Pseudocode:**
```
FOR ALL recordBatch WHERE isBugCondition(recordBatch) DO
  result := decodeRecordBatches_fixed(recordBatch)
  ASSERT result.Records is correctly decoded
  ASSERT result.Records.length > 0
  ASSERT result.Records[0].Value is valid decompressed data
END FOR
```

### Preservation Checking

**Goal**: Verify that for all inputs where the bug condition does NOT hold, the fixed function produces the same result as the original function.

**Pseudocode:**
```
FOR ALL recordBatch WHERE NOT isBugCondition(recordBatch) DO
  ASSERT decodeRecordBatches_original(recordBatch) = decodeRecordBatches_fixed(recordBatch)
END FOR
```

**Testing Approach**: Property-based testing is recommended for preservation checking because:
- It generates many test cases automatically across the input domain
- It catches edge cases that manual unit tests might miss
- It provides strong guarantees that behavior is unchanged for all non-buggy inputs

**Test Plan**: Observe behavior on UNFIXED code first for uncompressed and gzip-compressed batches, then write property-based tests capturing that behavior.

**Test Cases**:
1. **Uncompressed Preservation**: Observe that compression=0 batches decode correctly on unfixed code, then verify this continues after fix
2. **Gzip Preservation**: Observe that compression=1 batches decompress and decode correctly on unfixed code, then verify this continues after fix
3. **Batch Header Preservation**: Observe that all batch header fields are parsed correctly on unfixed code, then verify this continues after fix
4. **Error Handling Preservation**: Observe that gzip decompression errors are handled gracefully on unfixed code, then verify this continues after fix

### Unit Tests

- Test snappy decompression with valid compressed data
- Test lz4 decompression with valid compressed data
- Test zstd decompression with valid compressed data
- Test that uncompressed batches continue to work
- Test that gzip batches continue to work
- Test error handling for invalid compressed data

### Property-Based Tests

- Generate random record batches with each compression type and verify correct decompression
- Generate random uncompressed and gzip batches and verify preservation of existing behavior
- Test that all batch header fields are preserved across compression types

### Integration Tests

- Test full Kafka message flow with snappy-compressed batches
- Test full Kafka message flow with lz4-compressed batches
- Test full Kafka message flow with zstd-compressed batches
- Test mixed batches with different compression types in the same response
- Test that existing gzip and uncompressed flows continue to work end-to-end
