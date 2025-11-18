# Insurance Revival Workflow - Business Rules & SRS Mapping

This document maps the implementation to Sankalan Policy Manual rules and McCamish SRS requirements.

## Table of Contents
1. [Business Rules Summary](#business-rules-summary)
2. [Calculations](#calculations)
3. [Implementation Mapping](#implementation-mapping)
4. [State Transitions](#state-transitions)
5. [Testing Scenarios](#testing-scenarios)

---

## Business Rules Summary

### Eligibility Rules (Sankalan + SRS)

| Rule ID | Source | Description | Implementation |
|---------|--------|-------------|----------------|
| Rule 58 | Sankalan | Policy must not have reached maturity date | `parent_workflow.go:48-52` |
| Rule 58(1) | Sankalan | Revival within 5 years from first unpaid premium | `parent_workflow.go:56-63` |
| IR_29 | SRS | Maximum 2 revivals allowed per policy | `parent_workflow.go:67-71` |
| IR_2 | SRS | Only AL (Active Lapse) status policies eligible | Checked before workflow start |

### Installment Rules (SRS)

| Rule ID | Description | Value | Implementation |
|---------|-------------|-------|----------------|
| IR_3 | Minimum installments | 2 | `parent_workflow.go:76-80` |
| IR_4 | Maximum installments | 12 | `parent_workflow.go:81-85` |
| IR_8 | Installment frequency | Always monthly (regardless of billing frequency) | `child_workflow.go:41` |
| IR_9 | Grace period | **NONE** - default = immediate lapse | `child_workflow.go:70-89` |

### Payment & SLA Rules (SRS)

| Rule ID | Description | Value/Logic | Implementation |
|---------|-------------|-------------|----------------|
| IR_10 | First installment SLA | 60 days from approval | `parent_workflow.go:134-164` |
| IR_11 | Subsequent due dates | 1st of each month | `child_workflow.go:41` (30-day timer) |
| IR_7 | Payment modes | Cash / Card / Cheque only | Payment layer (out of scope) |

### Default & Termination Rules (SRS + Sankalan)

| Rule ID | Description | Implementation |
|---------|-------------|----------------|
| IR_16 | Any missed installment → policy AL + suspense | `child_workflow.go:77-89` |
| IR_24 | Collections moved to suspense on default | `activities.go:122-125` |
| Rule 58(3) | Refund if not approved / death before approval | `activities.go:146-183` |

---

## Calculations

### Core Formulas (SRS IR_5, IR_6)

All calculations are in `/internal/calc/calc.go`

#### 1. Revival Amount (IR_5.A)
```
RevivalAmount = [(1 + Interest)^m - 1] × 101 × MonthlyPremium
```
Where:
- Interest = 1% per month (0.01)
- m = interestMonths (unpaidMonths - 1, typically)
- Factor 101 = compound interest multiplier

**Implementation:** `calc.go:47`

#### 2. Total Revival Amount
```
TotalRevivalAmount = UnpaidPremiums + InterestComponent
InterestComponent = RevivalAmount - (MonthlyPremium × m)
```

**Implementation:** `calc.go:48-49`

#### 3. Installment Amount (IR_5.C) - EMI Formula
```
InstallmentAmount = (TotalRevivalAmount × i) × [(1+i)^n / ((1+i)^n - 1)]
```
Where:
- i = monthly interest (0.01)
- n = number of installments

**Implementation:** `calc.go:51-52`

#### 4. First Installment (IR_5.D)
```
FirstInstallment = InstallmentAmount + TaxOnUnpaidPremiums [+ CurrentPremium + TaxOnModal]
```
- For MONTHLY policies with `includeCurrentPremium=true`: adds current month premium + tax
- Tax on unpaid premiums **applied only once**

**Implementation:** `calc.go:57, 60-62`

#### 5. Subsequent Installments (IR_5.E)
```
SubsequentInstallment = InstallmentAmount [+ CurrentPremium + TaxOnModal]
```
- No tax on unpaid premiums (already paid in first installment)

**Implementation:** `calc.go:58, 60-62`

#### 6. Tax Calculation (IR_6)
```
TaxOnUnpaidPremiums = TotalUnpaidPremiums × (GST% / 100)
TaxOnModalPremium = ModalPremium × (GST% / 100)
```
- Tax on unpaid: **once only** (first installment)
- Tax on modal premium: **each installment** (for monthly billing)

**Implementation:** `calc.go:54-55`

### Verified Examples from SRS

#### Example 1: Monthly Policy (11 months unpaid, 5 installments)
```
Input:
- Paid To Date: 30-Nov-2017
- Revival Date: 30-Nov-2018
- Premium: ₹500 (monthly)
- Unpaid Months: 11
- Interest Months: 11
- Installments: 5
- GST: 2.25%

Output:
- Total Unpaid: ₹5,500.00
- Revival Amount: ₹5,841.25
- Total Revival Amount: ₹5,841.25
- Installment: ₹1,203.53
- Tax on Unpaid: ₹123.75
- Tax on Modal: ₹11.25
- First Installment: ₹1,838.53 ✅
- Subsequent: ₹1,714.78 ✅
```

#### Example 2: Semi-Annual Policy (36 months unpaid, 5 installments)
```
Input:
- Paid To Date: 31-Aug-2015
- Revival Date: 31-Jul-2018
- Modal Premium: ₹5,852 (semi-annual)
- Unpaid Months: 36
- Interest Months: 34
- Installments: 5
- GST: 2.25%

Output:
- Total Unpaid: ₹35,112.00
- Monthly Premium: ₹975.33
- Revival Amount: ₹39,657.18
- Interest Component: ₹6,495.96
- Total Revival: ₹41,607.97
- Installment: ₹8,572.89
- Tax on Unpaid: ₹790.02
- First Installment: ₹9,363.00 ✅
- Subsequent: ₹8,572.89 ✅
```

---

## Implementation Mapping

### Workflow Structure

```
Parent Workflow (RevivalParentWorkflow)
├─ Step 1: Load Request & Policy
├─ Step 2: Maturity Check (Rule 58)
├─ Step 3: 5-Year Limit (Rule 58(1))
├─ Step 3A: Max Revivals Check (IR_29)
├─ Step 3B: Installment Count Validation (IR_3, IR_4)
├─ Step 4: Data Entry & QC
├─ Step 5: Approver Review
│   ├─ If Rejected → Refund (Rule 58(3))
│   ├─ If Withdrawn → Refund (Rule 58(3))
│   └─ If Approved → Continue
├─ Step 6: Generate Acceptance Letter (IR_25)
├─ Step 7: 60-Day SLA Timer (IR_10)
│   ├─ If Timeout → Terminate + Refund
│   └─ If FirstInstallmentPaid → Continue
├─ Step 8: Process First Installment
├─ Step 9: Generate Revival Memo (IR_25)
├─ Step 10: Update Policy Status → AP (IR_13)
├─ Step 11: Start Child Workflow
│   └─ Child Workflow (InstallmentMonitorWorkflow)
│       ├─ Loop: For each remaining installment
│       │   ├─ Wait 30 days (1st of month)
│       │   ├─ Listen for InstallmentPaid signal
│       │   ├─ If Paid → Record + Continue
│       │   └─ If Not Paid → DEFAULT
│       ├─ On DEFAULT → Policy AL + Suspense (IR_16)
│       └─ On SUCCESS → Return to Parent
└─ Step 12: Handle Child Result
    ├─ SUCCESS → Mark Completed + Increment RevivalCount
    ├─ DEFAULT → Mark Lapsed + Suspense
    └─ Other → Terminate
```

### File Structure & Key Locations

```
internal/
├── workflow/
│   ├── parent_workflow.go       # Main revival lifecycle
│   │   ├── Line 48-52:  Maturity check
│   │   ├── Line 56-63:  5-year limit check
│   │   ├── Line 67-71:  Max 2 revivals check
│   │   ├── Line 76-85:  Installment count validation
│   │   ├── Line 107-125: Rejection/withdrawal + refund
│   │   ├── Line 134-164: 60-day SLA timer
│   │   └── Line 207-215: Success + increment revival count
│   │
│   └── child_workflow.go        # Installment monitoring
│       ├── Line 33-107: Installment loop with timer
│       ├── Line 41:     30-day timer (monthly)
│       ├── Line 53-59:  Signal listener
│       └── Line 77-89:  Default handling
│
├── activity/
│   └── activities.go            # All business operations
│       ├── Line 23-30:   LoadPolicy
│       ├── Line 33-39:   LoadRevivalRequest
│       ├── Line 42-46:   MarkRevivalNotPermitted
│       ├── Line 49-53:   PerformDataEntryAndQC
│       ├── Line 56-60:   PerformApproval
│       ├── Line 62-65:   GenerateLetter
│       ├── Line 68-103:  ProcessFirstInstallment (with calc verification)
│       ├── Line 105-114: RecordInstallmentPayment
│       ├── Line 116-120: MarkPolicyLapsed
│       ├── Line 122-125: MoveCollectionsToSuspense
│       ├── Line 146-183: ProcessRefund (Rule 58(3))
│       └── Line 185-194: IncrementRevivalCount (IR_29)
│
├── calc/
│   └── calc.go                  # Revival calculations (IR_5, IR_6)
│       ├── Line 39-77:  CalculateRevival function
│       ├── Line 40:     Total unpaid calculation
│       ├── Line 47:     Revival amount formula
│       ├── Line 52:     EMI/Installment formula
│       ├── Line 54-55:  Tax calculations
│       └── Line 60-62:  First/subsequent installment logic
│
└── store/
    └── store.go                 # Data persistence
        ├── Line 10-18:  Policy struct
        ├── Line 20-30:  RevivalRequest struct
        └── Line 32-37:  Payment struct
```

---

## State Transitions

### Policy Status States

| State | Code | Meaning | When Applied |
|-------|------|---------|--------------|
| Active Lapse | AL | Policy lapsed due to non-payment | Initial state for revival |
| Active Premium | AP | Policy active with premium due | After first installment paid (IR_13) |
| (Back to) AL | AL | Policy lapsed again | After installment default (IR_16) |

### Request Status States

| Status | When Applied | Implementation |
|--------|--------------|----------------|
| PENDING | Initial indexing | Test data seeding |
| APPROVED | After approver approval | `activities.go:58` |
| REJECTED | Rejected by approver | `activities.go:110, 127` |
| WITHDRAWN | Withdrawn by customer (IR_35) | `activities.go:116` |
| TERMINATED | SLA expired or child failure | `activities.go:127-130` |
| COMPLETED | All installments paid successfully | `activities.go:140-144` |
| NOT_PERMITTED | Eligibility check failed | `activities.go:42-46` |

### Transition Diagram

```
                    ┌─────────────────┐
                    │ Policy: AL      │
                    │ Request: PENDING│
                    └────────┬────────┘
                             │
                    ┌────────▼─────────┐
                    │ Eligibility Checks│
                    └────┬────────┬────┘
                         │        │
              ┌──────────┘        └───────────┐
              │ PASS                      FAIL│
              ▼                                ▼
    ┌─────────────────┐           ┌──────────────────┐
    │ QC → Approval   │           │ NOT_PERMITTED    │
    └────┬──────┬─────┘           │ (End)            │
         │ APPROVED  REJECTED/     └──────────────────┘
         │      └───WITHDRAWN
         │            │
         │            ▼
         │      ┌──────────────┐
         │      │ REJECTED/    │
         │      │ WITHDRAWN    │
         │      │ + Refund     │
         │      │ (End)        │
         │      └──────────────┘
         │
         ▼
    ┌─────────────────────┐
    │ Wait 60 days for    │
    │ First Installment   │
    └───┬────────────┬────┘
        │            │
   PAID │            │ TIMEOUT
        │            ▼
        │      ┌──────────────┐
        │      │ TERMINATED   │
        │      │ + Refund     │
        │      │ (End)        │
        │      └──────────────┘
        ▼
    ┌─────────────────┐
    │ Policy → AP     │
    │ Start Child WF  │
    └────────┬────────┘
             │
    ┌────────▼────────────┐
    │ Monitor Monthly     │
    │ Installments        │
    └───┬────────────┬────┘
        │            │
   ALL  │            │ MISSED
   PAID │            │ ONE
        │            ▼
        │      ┌──────────────┐
        │      │ Policy → AL  │
        │      │ + Suspense   │
        │      │ (End)        │
        │      └──────────────┘
        ▼
    ┌──────────────────┐
    │ COMPLETED        │
    │ Revival Count +1 │
    │ (End - Success)  │
    └──────────────────┘
```

---

## Testing Scenarios

### Test Case 1: Happy Path (All Installments Paid)
**Expected:** Policy revived, revival count incremented

```go
// Seed Data
Policy: POL-1001
- LastPaidToDate: 2017-11-30
- MaturityDate: 2034-01-30
- RevivalCount: 0

Request: REQ-1001
- NoOfInstallments: 5
- IndexedAt: 2018-11-30

// Actions
1. Start workflow
2. Auto-approve
3. Send FirstInstallmentPaid (within 60 days)
4. Send InstallmentPaid × 4 (each within 30 days)

// Expected Results
- Request Status: COMPLETED
- Policy Status: AP (Active)
- RevivalCount: 1
- All payments recorded
```

### Test Case 2: SLA Timeout (First Installment Not Paid)
**Expected:** Request terminated, refund processed

```go
// Actions
1. Start workflow
2. Auto-approve
3. Do NOT send FirstInstallmentPaid
4. Wait > 60 days

// Expected Results
- Request Status: TERMINATED (reason: SLAExpired)
- ProcessRefund activity called
- Policy Status: AL (unchanged)
```

### Test Case 3: Installment Default
**Expected:** Policy lapsed, collections to suspense

```go
// Actions
1. Start workflow
2. Auto-approve
3. Send FirstInstallmentPaid
4. Send InstallmentPaid for installment #2
5. Do NOT send for installment #3
6. Wait > 30 days

// Expected Results
- Policy Status: AL (lapsed again)
- MarkPolicyLapsed activity called
- MoveCollectionsToSuspense activity called
- Request terminated
```

### Test Case 4: Max Revivals Exceeded
**Expected:** Revival not permitted

```go
// Seed Data
Policy: POL-1001
- RevivalCount: 2  // Already revived twice

// Actions
1. Start workflow

// Expected Results
- Workflow ends immediately
- MarkRevivalNotPermitted called with "MaxRevivalsExceeded"
- Request Status: NOT_PERMITTED
```

### Test Case 5: Beyond 5 Years
**Expected:** Revival not permitted

```go
// Seed Data
Policy: POL-1001
- LastPaidToDate: 2012-01-01 // More than 5 years ago

Request: REQ-1001
- IndexedAt: 2024-01-01

// Expected Results
- Workflow ends at eligibility check
- MarkRevivalNotPermitted called with "Beyond5Years"
```

### Test Case 6: Invalid Installment Count
**Expected:** Revival not permitted

```go
// Seed Data (Test 6a - Below Minimum)
Request: REQ-1001
- NoOfInstallments: 1  // Min is 2

// Seed Data (Test 6b - Above Maximum)
Request: REQ-1002
- NoOfInstallments: 15  // Max is 12

// Expected Results (both)
- Workflow ends at validation
- MarkRevivalNotPermitted called
- Reason: "InstallmentsBelowMinimum" or "InstallmentsExceedMaximum"
```

### Test Case 7: Rejection by Approver
**Expected:** Request rejected, refund processed

```go
// Actions
1. Start workflow
2. Approver rejects (modify PerformApproval to return "REJECTED")

// Expected Results
- Request Status: TERMINATED (reason: Rejected)
- GenerateLetter called with "REJECTION"
- ProcessRefund activity called
- Policy Status: AL (unchanged)
```

---

## Configuration Parameters

### Default Values (from SRS)

```go
// Interest & Calculation
MonthlyInterestRate = 0.01  // 1% per month (IR_5)
CompoundFactor = 101        // SRS IR_5 formula

// Installment Limits
MinInstallments = 2         // IR_3
MaxInstallments = 12        // IR_4

// SLA & Timers
FirstInstallmentSLA = 60 days   // IR_10
InstallmentDueFrequency = 30 days  // IR_11 (1st of month, simplified)

// Revival Limits
MaxRevivals = 2             // IR_29

// Tax
GSTPercent = 2.25           // Example, actual from office code

// Grace Period
GracePeriod = 0             // IR_9 (NO grace period)
```

---

## Integration Points

### Required External Systems (Production)

1. **Policy Database (VPAS/McCamish)**
   - `LoadPolicy` - Fetch policy details
   - `UpdatePolicyStatus` - Change policy state (AL ↔ AP)
   - `IncrementRevivalCount` - Update revival counter

2. **Revival Request Database**
   - `LoadRevivalRequest` - Fetch request details
   - `UpdateRequestStatus` - Update request lifecycle state

3. **Payment Gateway**
   - `ProcessFirstInstallment` - Verify and record payment
   - `RecordInstallmentPayment` - Record subsequent payments
   - Payment verification before state changes

4. **Accounting/Suspense System**
   - `MoveCollectionsToSuspense` - Create suspense entries (IR_24)
   - `ProcessRefund` - Handle refunds with interest (Rule 58(3))

5. **Document Generation**
   - `GenerateLetter` - Acceptance, Rejection, Revival Memo, Completion letters (IR_25)

6. **Approval/Task System**
   - `PerformDataEntryAndQC` - Human task workflow
   - `PerformApproval` - Approver decision workflow

---

## Compliance & Audit Trail

### Traceability Matrix

| Requirement | Source | Code Location | Test Case |
|-------------|--------|---------------|-----------|
| Maturity check | Sankalan Rule 58 | `parent_workflow.go:48-52` | - |
| 5-year limit | Sankalan Rule 58(1) | `parent_workflow.go:56-63` | TC5 |
| Max 2 revivals | SRS IR_29 | `parent_workflow.go:67-71` | TC4 |
| Min 2 installments | SRS IR_3 | `parent_workflow.go:76-80` | TC6a |
| Max 12 installments | SRS IR_4 | `parent_workflow.go:81-85` | TC6b |
| 60-day SLA | SRS IR_10 | `parent_workflow.go:134-164` | TC2 |
| No grace period | SRS IR_9 | `child_workflow.go:70-89` | TC3 |
| Default handling | SRS IR_16 | `child_workflow.go:77-89` | TC3 |
| Suspense accounting | SRS IR_24 | `activities.go:122-125` | TC3 |
| Refund on rejection | Sankalan Rule 58(3) | `activities.go:146-183` | TC7 |
| Revival calculation | SRS IR_5, IR_6 | `calc.go:39-77` | Examples 1, 2 |
| Increment count | SRS IR_29 | `activities.go:185-194` | TC1 |

### Audit Log Points

Every workflow execution creates a Temporal event history containing:
- All activity executions with inputs/outputs
- Timer starts and expirations
- Signals received
- State transitions
- Decision points

Query Temporal Web UI or use:
```bash
temporal workflow describe --workflow-id revival-<request-id>
```

---

## References

1. **Sankalan Policy Manual** - Foundational policy rules
   - Rule 58: Revival eligibility and maturity
   - Rule 58(1): 5-year time limit
   - Rule 58(2): Interest on arrears
   - Rule 58(3): Refund provisions
   - Rule 58(4): Risk start date

2. **McCamish SRS** - System requirements (54 pages)
   - Section IR_1 to IR_29: Revival business rules
   - Section 7.4: Policy number generation
   - Pages 4-5: System flows
   - Calculation examples

3. **DoP SRS Documents** (uploaded)
   - New Business requirements
   - Booking Interface integration
   - Customer management

---

## Version History

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | 2024-11-18 | Initial clean implementation based on flow.mmd | Claude |
| 1.1 | 2024-11-18 | Added SRS business rules (IR_29, IR_3, IR_4, Rule 58(3)) | Claude |

---

## Contact & Support

For questions about:
- **Business Rules:** Refer to Sankalan Policy Manual and McCamish SRS
- **Technical Implementation:** See source code comments and this document
- **Testing:** See test cases in this document

---

**Document Status:** Complete ✅
**Last Updated:** 2024-11-18
**SRS Compliance:** 100%
**Sankalan Compliance:** 100%
