package fiberadapter

import "context"

type stubRPCNode struct {
	invoiceAddress string
	invoiceStatus  string
	sendPaymentID  string
	err            error
}

func (n *stubRPCNode) CreateInvoice(_ context.Context, _, _ string) (string, error) {
	if n.err != nil {
		return "", n.err
	}
	if n.invoiceAddress != "" {
		return n.invoiceAddress, nil
	}
	return "inv_stub_1", nil
}

func (n *stubRPCNode) GetInvoiceStatus(_ context.Context, _ string) (string, error) {
	if n.err != nil {
		return "", n.err
	}
	if n.invoiceStatus != "" {
		return n.invoiceStatus, nil
	}
	return "UNPAID", nil
}

func (n *stubRPCNode) ValidatePaymentRequest(_ context.Context, _ string) error {
	return n.err
}

func (n *stubRPCNode) SendPayment(_ context.Context, _, _, _, _ string) (string, error) {
	if n.err != nil {
		return "", n.err
	}
	if n.sendPaymentID != "" {
		return n.sendPaymentID, nil
	}
	return "tx_stub_1", nil
}
