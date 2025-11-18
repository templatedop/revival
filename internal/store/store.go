package store

import (
	"errors"
	"sync"
	"time"
)

// Simple in-memory store for prototype
type Policy struct {
	PolicyNumber     string
	IssueDate        time.Time
	MaturityDate     *time.Time
	PremiumFrequency string
	ModalPremium     float64
	LastPaidToDate   time.Time
	RevivalCount     int
}

type RevivalRequest struct {
	RequestID        string
	PolicyNumber     string
	IndexedAt        time.Time
	NoOfInstallments int
	GSTPercent       float64
	MonthlyInterest  float64
	InterestMonths   int
	UnpaidMonths     int
	Status           string // PENDING, APPROVED, TERMINATED, REJECTED, COMPLETED
}

type Payment struct {
	RequestID string
	Amount    float64
	When      time.Time
	ReceiptID string
}

type InMemoryStore struct {
	mtx        sync.RWMutex
	policies   map[string]Policy
	requests   map[string]RevivalRequest
	payments   map[string][]Payment // by requestID
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		policies: make(map[string]Policy),
		requests: make(map[string]RevivalRequest),
		payments: make(map[string][]Payment),
	}
}

// Policy operations
func (s *InMemoryStore) SavePolicy(p Policy) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.policies[p.PolicyNumber] = p
}

func (s *InMemoryStore) GetPolicy(policyNo string) (Policy, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	p, ok := s.policies[policyNo]
	if !ok {
		return p, errors.New("policy not found")
	}
	return p, nil
}

// Request operations
func (s *InMemoryStore) SaveRequest(r RevivalRequest) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.requests[r.RequestID] = r
}

func (s *InMemoryStore) GetRequest(reqID string) (RevivalRequest, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	r, ok := s.requests[reqID]
	if !ok {
		return r, errors.New("request not found")
	}
	return r, nil
}

func (s *InMemoryStore) UpdateRequestStatus(reqID, status string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	r, ok := s.requests[reqID]
	if !ok {
		return errors.New("request not found")
	}
	r.Status = status
	s.requests[reqID] = r
	return nil
}

// Payment
func (s *InMemoryStore) RecordPayment(p Payment) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.payments[p.RequestID] = append(s.payments[p.RequestID], p)
}

func (s *InMemoryStore) GetPaymentsByRequest(reqID string) []Payment {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.payments[reqID]
}
