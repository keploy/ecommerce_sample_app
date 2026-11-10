# Security Scan Report

**Scan date:** 10 November 2026
**Tool:** Gitleaks 8.30.1
**Repository:** ecommerce_sample_app

## 1. Scan Summary

| Scan scope | Detections | Result |
|---|---:|---|
| Git history | 771 | Findings require review |
| Current directory | 416 | Findings require review |
| Terraform directory | 0 | No detections |

### Current directory findings

| Rule | Count |
|---|---:|
| JWT | 374 |
| Generic API key | 42 |
| **Total** | **416** |

The 416 current-directory detections were located in Keploy fixture files. No detections were found outside the Keploy directory.

## 2. JWT Findings

The current-directory scan reported 374 JWT detections.

A separate local inspection identified 15 unique JWT strings:
- All 15 had standard JWT structure.
- All 15 had `exp` and `sub` claims.
- All 15 were expired at the time of inspection.
- No `iss` claim was present.

**Important:** Expiration at scan time does not prove that a token was never valid or never exposed. The original issuer and token provenance have not been independently verified. These findings must not be treated as confirmed false positives.

## 3. Generic API-Key Findings

The scan reported 42 generic API-key detections:

| Finding context | Count |
|---|---:|
| Postman-Token fields | 30 |
| Idempotency-Key fields | 7 |
| query fields in Keploy mocks | 5 |
| **Total** | **42** |

These detections were located in Keploy fixture files. The five `query` matches contained SQL keywords in their source lines.

The findings were introduced in these commits:
- `b9b12b5` — ci: add keploy tests
- `581b9f1` — feat: add coverage calculation

Fixture context suggests these may be test-data matches, but they have not been conclusively verified as false positives.

## 4. Terraform Scan

The Terraform directory scan returned zero detections.

Terraform state files, plans, variable files and local scan reports should remain excluded from Git where appropriate. Never commit cloud credentials or Terraform state containing sensitive values.

## 5. Report Handling

- Redacted Gitleaks JSON reports are excluded using `.gitleaks-*.json` in `.gitignore`.
- The temporary unredacted JWT report was deleted after local inspection.
- Unredacted reports must not be committed, uploaded or shared.

## 6. Recommended Remediation

1. Verify whether any detected JWTs or API tokens were ever issued as live credentials.
2. Revoke or rotate any credential confirmed to have been exposed.
3. Replace sensitive values in test fixtures with clearly synthetic test data.
4. Re-run Gitleaks after remediation and review any remaining findings.
5. Add Gitleaks scanning to the CI pipeline.
6. Document any approved allowlist exceptions with a clear justification.

## 7. Conclusion

Gitleaks identified historical and current-directory findings that require review. The Terraform scan returned no detections. Although the current-directory findings are confined to Keploy fixtures, their presence alone is not sufficient to classify them as harmless.

This report records the scan results and outstanding verification work. It does not claim that the repository is completely free of exposed secrets.
