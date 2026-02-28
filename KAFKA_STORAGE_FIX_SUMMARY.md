# Kafka Mock Storage Fix - Summary of Changes

## Problem
Kafka mocks are being stored as `kind: Generic` instead of `kind: Kafka` in YAML files.

## Root Cause Analysis
The issue has multiple layers:
1. OSS Kafka integration existed but was disabled (MatchType returned false)
2. ENT Kafka integration was trying to override but had conflicts
3. Storage layer needed Kafka-specific schema support
4. Priority conflicts between Kafka and Generic integrations

## Changes Made

### 1. OSS Kafka Schema Support
**File**: `keploy/pkg/models/kafka_schema.go`
- **Status**: ✅ Created
- **Purpose**: Define KafkaSchema struct for YAML storage
- **Content**: Similar to RedisSchema, with Kafka-specific fields

### 2. OSS Storage Layer
**File**: `keploy/pkg/platform/yaml/mockdb/util.go`
- **Status**: ✅ Updated
- **Changes**:
  - Added `case models.Kafka:` in `EncodeMock()` function
  - Added `case models.Kafka:` in `DecodeMocks()` function
  - Uses `KafkaSchema` for encoding/decoding

### 3. OSS Kafka Integration
**File**: `keploy/pkg/agent/proxy/integrations/kafka/kafka.go`
- **Status**: ✅ Updated
- **Changes**:
  - Fixed `MatchType()` to detect Kafka protocol (checks API key 0-67)
  - Increased Priority from 100 to 200 (higher than Generic)
  - Now properly detects Kafka traffic

### 4. OSS Kafka Recorder
**File**: `keploy/pkg/agent/proxy/integrations/kafka/recorder/recorder.go`
- **Status**: ✅ Updated
- **Changes**: Delegates to ENT recorder implementation
- **Code**: `return entRecorder.Record(ctx, logger, clientConn, destConn, mocks, opts)`

### 5. OSS Kafka Replayer
**File**: `keploy/pkg/agent/proxy/integrations/kafka/replayer/replayer.go`
- **Status**: ✅ Updated
- **Changes**: Delegates to ENT replayer implementation
- **Code**: `return entReplayer.Replay(ctx, logger, src, dstCfg, mockDb, opts)`

### 6. ENT Kafka Integration
**File**: `enterprise/pkg/core/proxy/integrations/kafka/kafka.go`
- **Status**: ✅ Updated (but not used)
- **Changes**: Removed unused wire import
- **Note**: This file is no longer imported/registered

### 7. ENT Parser Configuration
**File**: `enterprise/pkg/core/proxy/parsers.go`
- **Status**: ✅ Updated
- **Changes**: Removed Kafka import (Kafka now handled by OSS)

### 8. ENT Main
**File**: `enterprise/cmd/enterprise/main.go`
- **Status**: ✅ Updated
- **Changes**: Removed Kafka import

### 9. ENT Storage Overrides
**Files**: 
- `enterprise/pkg/platform/yaml/mockdb/db.go` - ✅ Deleted
- `enterprise/pkg/platform/yaml/mockdb/util.go` - ✅ Deleted
- **Purpose**: Removed ENT storage overrides to eliminate conflicts

## Expected Flow

1. **Traffic Detection**: OSS Kafka integration's `MatchType()` detects Kafka traffic (Priority 200)
2. **Recording**: OSS delegates to ENT recorder via `entRecorder.Record()`
3. **Mock Creation**: ENT recorder creates mock with:
   - `Kind: models.Kafka`
   - `KafkaRequests: []kafka.Request`
   - `KafkaResponses: []kafka.Response`
4. **Storage**: OSS `EncodeMock()` encodes using `KafkaSchema`
5. **YAML Output**: Mock stored as `kind: Kafka` with proper structure

## Current Status
❌ **Still Not Working** - Mocks are still being stored as `kind: Generic`

## Possible Remaining Issues

### Issue 1: Binary Not Rebuilt
- **Check**: Ensure the Keploy binary was rebuilt after changes
- **Action**: Run `go build` in both keploy and enterprise directories
- **Verify**: Check binary timestamp

### Issue 2: MatchType Not Being Called
- **Symptom**: Generic integration is being selected instead of Kafka
- **Possible Causes**:
  - Integration not registered properly
  - Priority not being respected
  - MatchType logic incorrect
- **Debug**: Add logging to MatchType function

### Issue 3: ENT Recorder Not Creating Kafka Mocks
- **Symptom**: ENT recorder might be creating Generic mocks
- **Check**: `enterprise/pkg/core/proxy/integrations/kafka/recorder/recorder.go`
- **Verify**: Line 280-290 should set `Kind: models.Kafka`

### Issue 4: Storage Layer Not Encoding Kafka
- **Symptom**: Kafka mocks being converted to Generic during storage
- **Check**: `keploy/pkg/platform/yaml/mockdb/util.go` line 242-256
- **Verify**: `case models.Kafka:` is being hit

### Issue 5: Integration Selection Logic
- **Location**: Proxy core that selects which integration to use
- **Issue**: Might not be respecting Priority or calling MatchType
- **Need**: Find where integrations are selected and verify logic

## Next Steps for Debugging

1. **Add Debug Logging**:
   ```go
   // In OSS kafka.go MatchType
   logger.Info("Kafka MatchType called", zap.Bool("result", result))
   
   // In ENT recorder recordMock
   logger.Info("Creating Kafka mock", zap.String("kind", string(kafkaMock.Kind)))
   
   // In OSS util.go EncodeMock
   logger.Info("EncodeMock called", zap.String("kind", string(mock.Kind)))
   ```

2. **Check Integration Registration**:
   - Verify `integrations.Registered` map contains Kafka
   - Check if OSS or ENT version is registered
   - Verify Priority value

3. **Trace Mock Creation**:
   - Add breakpoint or logging in ENT recorder
   - Verify `Kind: models.Kafka` is set
   - Check if mock reaches storage layer unchanged

4. **Find Integration Selection Code**:
   - Search for where `MatchType` is called
   - Find proxy code that iterates through integrations
   - Verify Priority-based selection logic

5. **Check for Double Registration**:
   - Both OSS and ENT might be registering Kafka
   - Last registration wins (map overwrite)
   - Verify only OSS registration is active

## Files to Investigate

1. **Proxy Core**: Find where integrations are selected
   - Search for: `integrations.Registered`, `MatchType`, `Priority`
   - Likely in: `keploy/pkg/agent/proxy/proxy.go` or similar

2. **Integration Selection**: 
   - How does proxy choose which integration to use?
   - Is Priority respected?
   - Is MatchType called for all integrations?

3. **Mock Flow**:
   - Trace from recorder → channel → storage
   - Check if mock is modified anywhere in between

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────┐
│                    OSS Kafka Integration                     │
│  - Registration (Priority 200)                              │
│  - MatchType (detects Kafka protocol)                       │
│  - Delegates to ENT for implementation                      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ├─ Record ──→ ENT Recorder
                            │              │
                            │              ├─ Creates Mock
                            │              │  Kind: Kafka
                            │              │  KafkaRequests
                            │              │  KafkaResponses
                            │              │
                            │              └─→ Mock Channel
                            │
                            └─ Replay ──→ ENT Replayer
                                           
┌─────────────────────────────────────────────────────────────┐
│                    OSS Storage Layer                         │
│  - EncodeMock (case models.Kafka)                           │
│  - Uses KafkaSchema                                          │
│  - Writes to YAML as kind: Kafka                            │
└─────────────────────────────────────────────────────────────┘
```

## Verification Checklist

- [ ] Binary rebuilt after changes
- [ ] OSS Kafka integration registered (check logs)
- [ ] MatchType being called for Kafka traffic
- [ ] ENT recorder creating Kafka mocks (not Generic)
- [ ] Storage layer receiving Kafka mocks
- [ ] EncodeMock hitting Kafka case
- [ ] YAML output shows `kind: Kafka`

## Contact Points for Further Investigation

The issue is likely in one of these areas:
1. Integration selection logic (proxy core)
2. Mock creation in ENT recorder
3. Mock transformation between recorder and storage
4. Binary not being rebuilt/redeployed
