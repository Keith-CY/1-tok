import { PortalShell } from "../../../../components/portal-shell";

export const dynamic = "force-dynamic";

export default async function CreateRFQPage() {
  return (
    <PortalShell
      eyebrow="Buyer portal / RFQ"
      title="Create a Request for Quote."
      copy="Describe the work you need done. Providers will submit bids with pricing and milestone breakdowns."
      signal="New RFQ"
    >
      <form method="POST" action="/buyer/rfqs/create/submit" className="space-y-6 max-w-2xl">
        <div>
          <label htmlFor="title" className="block text-sm font-medium mb-1">Title</label>
          <input id="title" name="title" type="text" required
            className="w-full border rounded px-3 py-2"
            placeholder="e.g., Agent runtime triage" />
        </div>

        <div>
          <label htmlFor="category" className="block text-sm font-medium mb-1">Category</label>
          <select id="category" name="category" required className="w-full border rounded px-3 py-2">
            <option value="">Select category</option>
            <option value="agent-ops">Agent Ops</option>
            <option value="agent-runtime">Agent Runtime</option>
            <option value="compute">Compute</option>
            <option value="data-pipeline">Data Pipeline</option>
          </select>
        </div>

        <div>
          <label htmlFor="scope" className="block text-sm font-medium mb-1">Scope</label>
          <textarea id="scope" name="scope" required rows={4}
            className="w-full border rounded px-3 py-2"
            placeholder="Describe the work scope, deliverables, and any constraints..." />
        </div>

        <div>
          <label htmlFor="budget" className="block text-sm font-medium mb-1">Budget (cents)</label>
          <input id="budget" name="budgetCents" type="number" required min={100}
            className="w-full border rounded px-3 py-2"
            placeholder="5000" />
          <p className="text-xs text-gray-500 mt-1">Enter amount in cents. 5000 = $50.00</p>
        </div>

        <div>
          <label htmlFor="deadline" className="block text-sm font-medium mb-1">Response Deadline</label>
          <input id="deadline" name="responseDeadlineAt" type="datetime-local" required
            className="w-full border rounded px-3 py-2" />
        </div>

        <div className="flex gap-3">
          <button type="submit" className="bg-blue-600 text-white px-6 py-2 rounded font-medium hover:bg-blue-700">
            Create RFQ
          </button>
          <a href="/buyer" className="px-6 py-2 border rounded text-gray-600 hover:bg-gray-50">
            Cancel
          </a>
        </div>
      </form>
    </PortalShell>
  );
}
