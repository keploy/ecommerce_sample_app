# Bugfix Requirements Document

## Introduction

The `decodeRecordBatches` function in `pkg/core/proxy/integrations/kafka/wire/bodies.go` currently only implements decompression for gzip-compressed Kafka record batches (compression type 1). However, the code recognizes and labels 5 compression types: none (0), gzip (1), snappy (2), lz4 (3), and zstd (4). When Kafka producers send record batches compressed with snappy, lz4, or zstd, the decoder fails to decompress them, resulting in corrupted or unreadable record data. This bug affects the proxy's ability to correctly decode and process Kafka messages using these compression algorithms.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN a record batch with compression type 2 (snappy) is received THEN the system fails to decompress the records data and attempts to decode compressed bytes as raw records

1.2 WHEN a record batch with compression type 3 (lz4) is received THEN the system fails to decompress the records data and attempts to decode compressed bytes as raw records

1.3 WHEN a record batch with compression type 4 (zstd) is received THEN the system fails to decompress the records data and attempts to decode compressed bytes as raw records

### Expected Behavior (Correct)

2.1 WHEN a record batch with compression type 2 (snappy) is received THEN the system SHALL decompress the records data using snappy decompression before decoding records

2.2 WHEN a record batch with compression type 3 (lz4) is received THEN the system SHALL decompress the records data using lz4 decompression before decoding records

2.3 WHEN a record batch with compression type 4 (zstd) is received THEN the system SHALL decompress the records data using zstd decompression before decoding records

2.4 WHEN decompression fails for any supported compression type THEN the system SHALL handle the error gracefully without crashing

### Unchanged Behavior (Regression Prevention)

3.1 WHEN a record batch with compression type 0 (none) is received THEN the system SHALL CONTINUE TO decode records without decompression

3.2 WHEN a record batch with compression type 1 (gzip) is received THEN the system SHALL CONTINUE TO decompress using gzip and decode records correctly

3.3 WHEN gzip decompression fails THEN the system SHALL CONTINUE TO handle the error gracefully as it currently does

3.4 WHEN record batches are successfully decoded THEN the system SHALL CONTINUE TO populate all RecordBatch fields correctly (BaseOffset, BatchLength, Compression, TimestampType, FirstTimestamp, MaxTimestamp, ProducerID, ProducerEpoch, BaseSequence, Records)
