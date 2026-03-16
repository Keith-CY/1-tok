import { PortalShell } from "../../../../components/portal-shell";

export const dynamic = "force-dynamic";

export default async function CreateRFQPage() {
  return (
    <PortalShell
      eyebrow="Buyer portal / RFQ"
      title="Create a Request for Quote."
      copy="Describe the work you need done. Providers will submit bids with pricing and milestone breakdowns."
      signal="New RFQ"
      asideTitle="Quick info"
      quickActions={[
        { label: "Back to buyer portal", href: "/buyer", tone: "secondary" },
        { label: "Open marketplace listings", href: "/buyer/listings", tone: "primary" },
      ]}
      asideItems={[]}
    >
      <form method="POST" action="/buyer/rfqs/create/submit" className="auth-form market-form max-w-2xl">
        <label className="auth-field">
          <span>Title</span>
          <input id="title" name="title" type="text" required placeholder="e.g., Agent runtime triage" />
        </label>

        <label className="auth-field">
          <span>Category</span>
          <select id="category" name="category" required>
            <option value="">Select category</option>
            <option value="agent-ops">Agent Ops</option>
            <option value="agent-runtime">Agent Runtime</option>
            <option value="compute">Compute</option>
            <option value="data-pipeline">Data Pipeline</option>
          </select>
        </label>

        <label className="auth-field">
          <span>Scope</span>
          <textarea
            id="scope"
            name="scope"
            required
            rows={4}
            placeholder="Describe the work scope, deliverables, and any constraints..."
          />
          <p className="text-xs text-gray-500 mt-1">Clear requirements reduce proposal ambiguity.</p>
        </label>

        <label className="auth-field">
          <span>Budget (cents)</span>
          <input
            id="budget"
            name="budgetCents"
            type="number"
            required
            min={100}
            placeholder="5000"
          />
          <p className="text-xs text-gray-500 mt-1">Enter amount in cents. 5000 = $50.00</p>
        </label>

        <label className="auth-field">
          <span>Response Deadline</span>
          <input id="deadline" name="responseDeadlineAt" type="datetime-local" required />
        </label>

        <div className="inline-form">
          <button type="submit" className="action-button">
            Create RFQ
          </button>
          <a href="/buyer" className="action-button inline-block">
            Cancel
          </a>
        </div>
      </form>
    </PortalShell>
  );
}
