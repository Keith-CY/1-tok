package platform

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

// toCSV is a tiny helper to keep CSV generation consistent and RFC-safe.
func toCSV(rows [][]string) (string, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)
	if err := w.WriteAll(rows); err != nil {
		return "", err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ExportOrdersCSV generates a CSV representation of orders.
func (a *App) ExportOrdersCSV() (string, error) {
	orders, err := a.orders.List()
	if err != nil {
		return "", err
	}

	rows := [][]string{{"OrderID", "BuyerOrgID", "ProviderOrgID", "Status", "FundingMode", "Milestones", "TotalBudgetCents", "TotalSettledCents"}}
	for _, o := range orders {
		var totalBudget, totalSettled int64
		for _, ms := range o.Milestones {
			totalBudget += ms.BudgetCents
			totalSettled += ms.SettledCents
		}
		rows = append(rows, []string{
			o.ID, o.BuyerOrgID, o.ProviderOrgID,
			string(o.Status), string(o.FundingMode),
			fmt.Sprintf("%d", len(o.Milestones)),
			fmt.Sprintf("%d", totalBudget),
			fmt.Sprintf("%d", totalSettled),
		})
	}

	return toCSV(rows)
}

// ExportDisputesCSV generates a CSV representation of disputes.
func (a *App) ExportDisputesCSV() (string, error) {
	disputes, err := a.disputes.List()
	if err != nil {
		return "", err
	}

	rows := [][]string{{"DisputeID", "OrderID", "MilestoneID", "Status", "Reason", "RefundCents", "CreatedAt"}}
	for _, d := range disputes {
		reason := d.Reason
		rows = append(rows, []string{
			d.ID, d.OrderID, d.MilestoneID,
			string(d.Status), reason, fmt.Sprintf("%d", d.RefundCents), d.CreatedAt.Format("2006-01-02"),
		})
	}

	return toCSV(rows)
}

// ExportRFQsCSV generates a CSV representation of RFQs.
func (a *App) ExportRFQsCSV() (string, error) {
	rfqs, err := a.rfqs.List()
	if err != nil {
		return "", err
	}

	rows := [][]string{{"RFQID", "BuyerOrgID", "Title", "Category", "BudgetCents", "Status", "CreatedAt"}}
	for _, r := range rfqs {
		title := r.Title
		rows = append(rows, []string{
			r.ID, r.BuyerOrgID, title, r.Category,
			fmt.Sprintf("%d", r.BudgetCents), string(r.Status), r.CreatedAt.Format("2006-01-02")})
	}
	return toCSV(rows)
}

// ExportProviderApplicationsCSV generates a CSV of provider applications.
func (a *App) ExportProviderApplicationsCSV() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	rows := [][]string{{"ApplicationID", "OrgID", "Name", "Status", "ReviewedBy", "SubmittedAt"}}
	for _, app := range a.providerApplications {
		rows = append(rows, []string{
			app.ID, app.OrgID, app.Name, string(app.Status),
			app.ReviewedBy, app.SubmittedAt.Format("2006-01-02")})
	}
	csv, err := toCSV(rows)
	if err != nil {
		return ""
	}
	return csv
}
