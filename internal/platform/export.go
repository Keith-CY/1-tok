package platform

import (
	"fmt"
	"strings"
)

// ExportOrdersCSV generates a CSV representation of orders.
func (a *App) ExportOrdersCSV() (string, error) {
	orders, err := a.orders.List()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("OrderID,BuyerOrgID,ProviderOrgID,Status,FundingMode,Milestones,TotalBudgetCents,TotalSettledCents\n")

	for _, o := range orders {
		var totalBudget, totalSettled int64
		for _, ms := range o.Milestones {
			totalBudget += ms.BudgetCents
			totalSettled += ms.SettledCents
		}
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%d,%d,%d\n",
			o.ID, o.BuyerOrgID, o.ProviderOrgID,
			o.Status, o.FundingMode,
			len(o.Milestones), totalBudget, totalSettled))
	}

	return sb.String(), nil
}

// ExportDisputesCSV generates a CSV representation of disputes.
func (a *App) ExportDisputesCSV() (string, error) {
	disputes, err := a.disputes.List()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("DisputeID,OrderID,MilestoneID,Status,Reason,RefundCents,CreatedAt\n")

	for _, d := range disputes {
		reason := strings.ReplaceAll(d.Reason, ",", ";")
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%d,%s\n",
			d.ID, d.OrderID, d.MilestoneID,
			d.Status, reason, d.RefundCents, d.CreatedAt.Format("2006-01-02")))
	}

	return sb.String(), nil
}

// ExportRFQsCSV generates a CSV representation of RFQs.
func (a *App) ExportRFQsCSV() (string, error) {
	rfqs, err := a.rfqs.List()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("RFQID,BuyerOrgID,Title,Category,BudgetCents,Status,CreatedAt\n")
	for _, r := range rfqs {
		title := strings.ReplaceAll(r.Title, ",", ";")
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d,%s,%s\n",
			r.ID, r.BuyerOrgID, title, r.Category,
			r.BudgetCents, r.Status, r.CreatedAt.Format("2006-01-02")))
	}
	return sb.String(), nil
}

// ExportProviderApplicationsCSV generates a CSV of provider applications.
func (a *App) ExportProviderApplicationsCSV() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("ApplicationID,OrgID,Name,Status,ReviewedBy,SubmittedAt\n")
	for _, app := range a.providerApplications {
		name := strings.ReplaceAll(app.Name, ",", ";")
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			app.ID, app.OrgID, name, app.Status,
			app.ReviewedBy, app.SubmittedAt.Format("2006-01-02")))
	}
	return sb.String()
}
