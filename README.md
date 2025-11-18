# Insurance Revival Workflow

This project implements a Temporal-based workflow system for insurance policy revival as per the flow diagram in `flow.mmd`.

## Overview

The revival workflow handles the complete lifecycle of an insurance policy revival request, from initial indexing through installment payments to completion.

### Workflow Architecture

The system consists of two main workflows:

1. **Parent Workflow** (`RevivalParentWorkflow`) - Manages the overall revival request lifecycle
2. **Child Workflow** (`InstallmentMonitorWorkflow`) - Monitors installment payments

## Flow Diagram Reference

The implementation follows the flow defined in `flow.mmd`:

### Parent Workflow Steps:
1. Index Revival Request (IR_2)
2. Check if policy has reached maturity date (Rule 58)
3. Check if within 5 years from first unpaid premium (Rule 58(1))
4. Data Entry and QC Verification
5. Approver Review
6. Generate Acceptance Letter (IR_25)
7. Start 60-Day SLA Timer (IR_10)
8. Wait for First Installment Payment signal
9. Generate Revival Memo (IR_25)
10. Update Policy Status to AP (IR_13)
11. Start Child Workflow for remaining installments
12. Handle child workflow result

### Child Workflow Steps:
1. Wait for next installment due date (1st of each month)
2. Check if installment paid
3. If paid: Record payment and continue
4. If not paid: Trigger default (IR_16), mark policy as AL, move to suspense (IR_24)
5. Return SUCCESS or DEFAULT to parent

## Project Structure

```
revival/
├── flow.mmd                    # Flow diagram
├── cmd/
│   ├── worker/main.go         # Worker that executes workflows
│   └── client/main.go         # Client to start and signal workflows
├── internal/
│   ├── workflow/
│   │   ├── parent_workflow.go # Parent workflow implementation
│   │   └── child_workflow.go  # Child workflow implementation
│   ├── activity/
│   │   └── activities.go      # All activity implementations
│   ├── store/
│   │   └── store.go           # In-memory data store
│   └── calc/
│       └── calc.go            # Revival calculation logic
└── README.md                   # This file
```

## Data Models

### Policy
- `PolicyNumber`: Unique policy identifier
- `IssueDate`: When policy was issued
- `MaturityDate`: When policy matures (nullable)
- `PremiumFrequency`: MONTHLY, QUARTERLY, SEMIANNUAL, YEARLY
- `ModalPremium`: Premium amount per frequency
- `LastPaidToDate`: Last date premium was paid
- `RevivalCount`: Number of times policy has been revived

### Revival Request
- `RequestID`: Unique request identifier
- `PolicyNumber`: Associated policy
- `IndexedAt`: When request was created
- `NoOfInstallments`: Number of installments (typically 5)
- `GSTPercent`: GST percentage (e.g., 2.25)
- `MonthlyInterest`: Interest rate per month (e.g., 0.01 = 1%)
- `InterestMonths`: Number of months for interest calculation
- `UnpaidMonths`: Total unpaid months
- `Status`: PENDING, APPROVED, TERMINATED, REJECTED, COMPLETED

## Activities

All activities are implemented in `internal/activity/activities.go`:

- `LoadPolicy` - Load policy data from store
- `LoadRevivalRequest` - Load revival request data
- `MarkRevivalNotPermitted` - Mark request as not permitted (maturity/5-year rule)
- `PerformDataEntryAndQC` - Data entry and QC verification
- `PerformApproval` - Approver review (auto-approved in demo)
- `GenerateLetter` - Generate letters (ACCEPTANCE, REJECTION, REVIVAL_MEMO, COMPLETION)
- `ProcessFirstInstallment` - Process and validate first installment payment
- `RecordInstallmentPayment` - Record subsequent installment payments
- `UpdatePolicyStatus` - Update policy status (e.g., to AP)
- `MarkPolicyLapsed` - Mark policy as lapsed (AL status)
- `MoveCollectionsToSuspense` - Move collections to suspense
- `MarkRequestTerminated` - Mark request as terminated with reason
- `MarkRequestCompleted` - Mark request as successfully completed

## Signals

The workflows respond to the following signals:

- `FirstInstallmentPaid` - Sent when first installment is paid (payload: amount, receipt)
- `InstallmentPaid` - Sent when subsequent installments are paid (payload: amount, receipt)

## Running the System

### Prerequisites

1. Go 1.23+ installed
2. Temporal server running locally (default: localhost:7233)

### Start Temporal Server

```bash
# Using Temporal CLI (recommended)
temporal server start-dev

# Or using Docker
docker run -p 7233:7233 temporalio/auto-setup:latest
```

### Run the Worker

The worker processes workflow tasks and executes activities:

```bash
go run cmd/worker/main.go
```

The worker will:
- Start listening on the `revival-task-queue`
- Seed test data (Policy POL-1001, Request REQ-1001)
- Register workflows and activities
- Wait for workflow executions

### Run the Client Demo

In a separate terminal, run the client to test the workflow:

```bash
go run cmd/client/main.go
```

The client will:
1. Start the parent workflow with test request REQ-1001
2. Wait 3 seconds for initial processing
3. Send the first installment payment signal
4. Send 4 subsequent installment payments (every 2-3 seconds)
5. Wait for workflow completion
6. Display success message

## Example Output

### Worker Output
```
Starting worker for task queue revival-task-queue...
Test data seeded: Policy POL-1001, Request REQ-1001
INFO  Starting Revival Parent Workflow requestID=REQ-1001
INFO  Loaded request and policy policyNumber=POL-1001
INFO  Performing data entry and QC verification
INFO  Approver auto-approved for prototype
INFO  Request approved - generating acceptance letter
INFO  Starting 60-day SLA timer for first installment payment
INFO  Received FirstInstallmentPaid signal
INFO  Starting child workflow for installment monitoring
INFO  All installments paid successfully
```

### Client Output
```
========== Revival Workflow Demo ==========
Request ID: REQ-1001
Policy Number: POL-1001

✓ Started Revival Parent Workflow
  Workflow ID: revival-abc123...

Simulating first installment payment...
✓ Sent FirstInstallmentPaid signal (Amount: ₹9,363.00)

✓ Sent InstallmentPaid signal #2 (Amount: ₹8,572.89)
✓ Sent InstallmentPaid signal #3 (Amount: ₹8,572.89)
✓ Sent InstallmentPaid signal #4 (Amount: ₹8,572.89)
✓ Sent InstallmentPaid signal #5 (Amount: ₹8,572.89)

========================================
✓ Revival Workflow Completed Successfully!
  All 5 installments paid
  Policy revival completed
  Request marked as COMPLETED
========================================
```

## Testing Different Scenarios

### Scenario 1: Successful Revival (Default)
Run the client as-is to test the happy path with all installments paid.

### Scenario 2: SLA Timeout
Modify the client to NOT send the first installment signal. The workflow will timeout after 60 days (simulated).

### Scenario 3: Installment Default
Modify the client to skip one of the installment payments. The child workflow will mark the policy as lapsed.

### Scenario 4: Maturity Check
Modify the test data in `cmd/worker/main.go` to set a maturity date in the past. The workflow will reject the revival.

### Scenario 5: 5-Year Limit Check
Modify the test data to set `LastPaidToDate` more than 5 years before the current date. The workflow will reject the revival.

## Workflow Execution Times (Demo)

For demonstration purposes, timers are shortened:
- 60-day SLA timer: Actually 60 days (can be shortened for testing)
- Monthly installment timer: Set to 30 days (can be shortened for testing)

In production, these would be set to actual business timescales.

## Monitoring Workflows

You can monitor workflow execution using the Temporal Web UI:

```bash
# Temporal Web UI is available at:
http://localhost:8233
```

Navigate to the workflow execution to see:
- Workflow history
- Event timeline
- Activity executions
- Signals received
- Current state

## Development Notes

### In-Memory Store
The current implementation uses an in-memory store for simplicity. In production:
- Replace with actual database (PostgreSQL, MySQL, etc.)
- Implement proper transaction handling
- Add persistence for payments and audit logs

### Activity Implementations
Current activities are simplified for demonstration:
- `PerformApproval` auto-approves (in production, would wait for human approval)
- `GenerateLetter` just logs (in production, would generate PDF/email)
- Policy status updates are logged only (in production, would update database)

### Error Handling
The workflows include:
- Retry policies for activities
- Error logging
- Graceful degradation for non-critical failures

### Extensibility
The implementation can be extended to support:
- Multiple revival types (lump sum vs installment)
- Dynamic installment schedules
- Partial payments
- Payment reversals
- Workflow queries for status checks

## References

- Flow Diagram: `flow.mmd`
- Temporal Documentation: https://docs.temporal.io/
- Revival Calculation: `internal/calc/calc.go`

## Clean Implementation

This is a fresh implementation based on the flow diagram. The following improvements were made:

1. **Removed Duplicates**: Cleaned up duplicate workflow files and type definitions
2. **Added Missing Activities**: Added `UpdatePolicyStatus` and `MarkRequestCompleted`
3. **Comprehensive Comments**: Each step references the flow diagram (IR_XX codes)
4. **Better Logging**: Added detailed logging at each workflow step
5. **Complete Demo**: Client demonstrates full workflow from start to completion
6. **Test Data Seeding**: Worker automatically seeds test data for easy testing
7. **Error Handling**: Proper error handling and retry policies

## Support

For issues or questions about the implementation, refer to:
1. The flow diagram in `flow.mmd`
2. Temporal documentation for workflow concepts
3. Code comments in the workflow files
