# 123pan Share Integration - Implementation Notes

## Overview
This document outlines the work needed to integrate 123pan's official share creation API with OpenList's sharing system, as requested in the issue.

## Current Status

### Completed
1. **Added share creation methods to 123pan driver** (`drivers/123/share.go`):
   - `CreateShare()`: Creates a share via 123pan API
   - `GetShareLink()`: Helper to get share link for a file
   - `Other()`: Interface method to support share creation

### What Still Needs to be Done

#### 1. API Endpoint Verification
- **File**: `drivers/123/share.go` 
- **Issue**: The API endpoints and request/response formats are based on assumptions
- **Action Needed**: 
  - Verify actual API endpoints from documentation
  - Update `ShareCreateReq` and `ShareCreateResp` structures to match actual API
  - Test with real 123pan account

#### 2. OpenList Sharing System Integration
- **Files**: `internal/sharing/`, `server/handles/sharing.go`
- **Current**: OpenList creates shares of files stored in any driver, but doesn't call driver-specific share creation
- **Needed Changes**:
  a. Detect when files being shared are from a 123pan storage
  b. Call `Pan123.Other(ctx, OtherArgs{Method: "create_share", ...})` to create official share
  c. Store the returned share URL (in `model.Sharing` or a new field)
  d. Use this stored share URL when generating download links

#### 3. Download Flow Modification  
- **File**: `server/handles/sharing.go` - `SharingDown()` function
- **Current**: Calls `op.Link()` which gets download URL from driver
- **Needed**: Check if share has an official platform share URL, redirect to it instead

#### 4. Frontend Changes
- **Not in this repository**: Frontend code needs updating
- **Changes Needed**:
  - When displaying file in share view, check if it has an official share URL
  - Show "Open in 123pan" or redirect button instead of/alongside download button
  - Handle password passing for 123pan shares

#### 5. Share Password Support
- **Files**: `internal/model/sharing.go`, `server/handles/sharing.go`
- **Current**: OpenList has its own share password system
- **Integration Needed**:
  - When creating 123pan share, optionally pass OpenList's share password to 123pan API
  - OR: Keep separate passwords (OpenList access + 123pan share access)
  - Document the behavior for users

#### 6. Share Expiration Sync
- **Issue**: OpenList shares and 123pan shares may have different expiration times
- **Solution Needed**:
  - Sync expiration when creating 123pan share
  - Handle case where 123pan share expires before OpenList share

## Testing Requirements

1. **Unit Tests**: Test share creation with mocked API responses
2. **Integration Tests**: Test with actual 123pan account
3. **End-to-End Tests**: 
   - Create OpenList share of 123pan file
   - Verify 123pan share is created
   - Access the share and verify redirect works
   - Test with/without passwords
   - Test expiration behavior

## Alternative Simpler Approach

If full integration is too complex, consider:
1. Add UI button "Create 123pan Share" for files in 123pan storage
2. Users explicitly create 123pan share when needed
3. Display both OpenList share link and 123pan share link
4. Users choose which to use

## Next Steps
1. Review 123pan API documentation to verify endpoints
2. Decide on integration approach (automatic vs manual)
3. Design the share metadata storage strategy
4. Implement OpenList sharing system hooks
5. Update frontend
6. Test thoroughly with real accounts
