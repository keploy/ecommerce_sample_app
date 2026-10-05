# Implementation Plan

- [x] 1. Write bug condition exploration test
  - **Property 1: Fault Condition** - Unsupported Compression Types Fail to Decompress
  - **CRITICAL**: This test MUST FAIL on unfixed code - failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior - it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate the bug exists
  - **Scoped PBT Approach**: Scope the property to concrete failing cases - record batches with compression types 2 (snappy), 3 (lz4), and 4 (zstd)
  - Test that decodeRecordBatches correctly decompresses and decodes records for compression types 2, 3, and 4
  - Generate test record batches with snappy, lz4, and zstd compression containing known record data
  - Assert that decoded records match the original uncompressed data (from Expected Behavior in design)
  - Run test on UNFIXED code
  - **EXPECTED OUTCOME**: Test FAILS (this is correct - it proves the bug exists)
  - Document counterexamples found (e.g., "snappy-compressed batch returns corrupted records", "lz4-compressed batch fails to decode")
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4_

- [ ] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** - Existing Compression Behavior Unchanged
  - **IMPORTANT**: Follow observation-first methodology
  - Observe behavior on UNFIXED code for non-buggy inputs (compression types 0 and 1)
  - Test compression type 0 (none): records decode correctly without decompression
  - Test compression type 1 (gzip): records decompress with gzip and decode correctly
  - Test gzip decompression error handling: malformed gzip data is handled gracefully
  - Test RecordBatch field population: all fields (BaseOffset, BatchLength, Compression, TimestampType, FirstTimestamp, MaxTimestamp, ProducerID, ProducerEpoch, BaseSequence, Records) are populated correctly
  - Write property-based tests capturing observed behavior patterns from Preservation Requirements
  - Property-based testing generates many test cases for stronger guarantees
  - Run tests on UNFIXED code
  - **EXPECTED OUTCOME**: Tests PASS (this confirms baseline behavior to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [ ] 3. Fix for unsupported Kafka compression types (snappy, lz4, zstd)

  - [ ] 3.1 Add compression library imports
    - Import "github.com/golang/snappy" for snappy decompression
    - Import "github.com/pierrec/lz4/v4" for lz4 decompression
    - Import "github.com/klauspost/compress/zstd" for zstd decompression
    - _Bug_Condition: isBugCondition(batch) where batch.compression ∈ {2, 3, 4}_
    - _Expected_Behavior: For compression type 2, decompress using snappy; for type 3, decompress using lz4; for type 4, decompress using zstd; handle decompression errors gracefully_
    - _Preservation: Compression types 0 and 1 continue to work as before; gzip error handling unchanged; RecordBatch fields populated correctly_
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 3.4_

  - [ ] 3.2 Extend decompression logic in decodeRecordBatches
    - Add case for compression == 2 (snappy): use snappy.Decode to decompress recordsData
    - Add case for compression == 3 (lz4): use lz4.NewReader to decompress recordsData
    - Add case for compression == 4 (zstd): use zstd.NewReader to decompress recordsData
    - Handle decompression errors gracefully for all new compression types (similar to gzip)
    - Ensure decompressed data replaces recordsData before calling decodeRecords
    - _Bug_Condition: isBugCondition(batch) where batch.compression ∈ {2, 3, 4}_
    - _Expected_Behavior: For compression type 2, decompress using snappy; for type 3, decompress using lz4; for type 4, decompress using zstd; handle decompression errors gracefully_
    - _Preservation: Compression types 0 and 1 continue to work as before; gzip error handling unchanged; RecordBatch fields populated correctly_
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 3.4_

  - [ ] 3.3 Verify bug condition exploration test now passes
    - **Property 1: Expected Behavior** - Unsupported Compression Types Now Decompress Correctly
    - **IMPORTANT**: Re-run the SAME test from task 1 - do NOT write a new test
    - The test from task 1 encodes the expected behavior
    - When this test passes, it confirms the expected behavior is satisfied
    - Run bug condition exploration test from step 1
    - **EXPECTED OUTCOME**: Test PASSES (confirms bug is fixed)
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [ ] 3.4 Verify preservation tests still pass
    - **Property 2: Preservation** - Existing Compression Behavior Unchanged
    - **IMPORTANT**: Re-run the SAME tests from task 2 - do NOT write new tests
    - Run preservation property tests from step 2
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions)
    - Confirm all tests still pass after fix (no regressions)
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [ ] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
