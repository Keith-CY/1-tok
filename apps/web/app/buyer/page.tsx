import { formatMoney } from "@1tok/contracts";

import { PortalShell } from "../../components/portal-shell";
import { SummaryCard } from "../../components/summary-card";
import { StatusBadge, ProgressBar, EmptyState } from "../../components/ui";
import { getBuyerDashboardData } from "../../lib/api";
import { requirePortalViewer } from "../../lib/viewer";

export const dynamic = "force-dynamic";

const PROGRESS_WARNING_THRESHOLD = 0.9;

export default async function BuyerPage({
  searchParams,
}: {
  searchParams?: Record<string, string | string[] | undefined>;
}) {
  const viewer = await requirePortalViewer("buyer", "/buyer");
  const data = await getBuyerDashboardData({
    authToken: viewer.token,
    buyerOrgId: viewer.membership.organization.id,
    requireLive: true,
  });

  const listingSearch = readSearchParam(searchParams, "listingSearch").toLowerCase();
  const listingCategory = (readSearchParam(searchParams, "listingCategory") || "all").toLowerCase();
  const rfqSearch = readSearchParam(searchParams, "rfqSearch").toLowerCase();
  const rfqStatusFilter = readSearchParam(searchParams, "rfqStatusFilter") || "all";
  const messageSearch = readSearchParam(searchParams, "messageSearch").toLowerCase();


  const chipClass = (active: boolean) =>
    active ? "action-button action-button--active" : "action-button";

  const buildListingCategoryHref = (nextCategory: string) => {
    const params = new URLSearchParams();

    if (listingSearch) {
      params.set("listingSearch", listingSearch);
    }

    if (nextCategory !== "all") {
      params.set("listingCategory", nextCategory);
    }

    if (rfqSearch) {
      params.set("rfqSearch", rfqSearch);
    }

    if (rfqStatusFilter && rfqStatusFilter !== "all") {
      params.set("rfqStatusFilter", rfqStatusFilter);
    }

    if (messageSearch) {
      params.set("messageSearch", messageSearch);
    }

    const queryString = params.toString();
    return queryString ? `/buyer?${queryString}` : "/buyer";
  };

  const buildRFQStatusHref = (nextStatus: string) => {
    const params = new URLSearchParams();

    if (listingSearch) {
      params.set("listingSearch", listingSearch);
    }

    if (listingCategory !== "all") {
      params.set("listingCategory", listingCategory);
    }

    if (rfqSearch) {
      params.set("rfqSearch", rfqSearch);
    }

    if (messageSearch) {
      params.set("messageSearch", messageSearch);
    }

    if (nextStatus !== "all") {
      params.set("rfqStatusFilter", nextStatus);
    }

    const queryString = params.toString();
    return queryString ? `/buyer?${queryString}` : "/buyer";
  };

  const listingCategories = Array.from(new Set(data.recommendedListings.map((listing) => listing.category.toLowerCase())).values()).sort();

  const filteredListings = data.recommendedListings.filter((listing) => {
    const listingCategoryMatch = listingCategory === "all" || listing.category.toLowerCase() === listingCategory;
    const searchMatch =
      listing.title.toLowerCase().includes(listingSearch) ||
      listing.category.toLowerCase().includes(listingSearch) ||
      listing.tags.some((tag) => tag.toLowerCase().includes(listingSearch));

    return listingCategoryMatch && searchMatch;
  });

  const filteredRFQs = data.rfqBook.filter((rfq) => {
    const matchesStatus =
      rfqStatusFilter === "all"
        ? true
        : rfqStatusFilter === "open"
          ? rfq.status === "open"
          : rfq.status === rfqStatusFilter;

    const matchesSearch = rfq.title.toLowerCase().includes(rfqSearch);

    return matchesStatus && matchesSearch;
  });

  const filteredMessages = data.inbox.filter((message) =>
    message.title.toLowerCase().includes(messageSearch) ||
    message.detail.toLowerCase().includes(messageSearch),
  );

  return (
    <PortalShell
      eyebrow="Buyer portal / orchestration budget"
      title="Buy agent work like a floor trader, not a ticket submitter."
      copy="This view keeps discovery, funding mode, milestone exposure, and pause recovery in one frame. Buyers should see exactly when Carrier requests more budget and what will happen if they ignore it."
      signal="Credit and prepaid capital share the same order frame"
      asideTitle="Buyer signal deck"
      quickActions={[
        { label: "Create RFQ", href: "#create-rfq", tone: "primary" },
        { label: "Open RFQ book", href: "#rfq-book", tone: "secondary" },
        { label: "Review inbox", href: "#message-inbox", tone: "secondary" },
      ]}
      asideItems={[
        { label: "Buyer org", value: data.summary.buyerOrgId, tone: "mint" },
        { label: "Open RFQs", value: `${data.summary.openRFQs}` },
        { label: "Paused orders", value: `${data.summary.pausedOrders}`, tone: "warning" },
      ]}
    >
      <div className="stat-grid">
        <SummaryCard
          kicker="Active orders"
          value={`${data.summary.activeOrders}`}
          hint="Orders currently executing under platform-controlled channels."
        />
        <SummaryCard
          kicker="Open RFQs"
          value={`${data.summary.openRFQs}`}
          hint="Buyer-authored requests still collecting bids or awaiting award."
        />
        <SummaryCard
          kicker="Paused orders"
          value={`${data.summary.pausedOrders}`}
          hint="Orders currently waiting on more budget before Carrier can continue."
        />
        <SummaryCard
          kicker="Available listings"
          value={`${data.summary.availableListings}`}
          hint="Listings currently visible in the marketplace catalog for this buyer session."
        />
      </div>

      <article className="feed-card" id="create-rfq">
        <span className="tag">Open an RFQ</span>
        <h3>Buyers should be able to turn intent into a priced market request immediately.</h3>
        <form className="auth-form market-form" action="/buyer/rfqs" method="post">
          <div className="market-form__grid">
            <label className="auth-field">
              <span>Title</span>
              <input name="title" type="text" placeholder="Need live carrier triage" required />
            </label>
            <label className="auth-field">
              <span>Category</span>
              <input name="category" type="text" defaultValue="agent-ops" required />
            </label>
            <label className="auth-field">
              <span>Budget cents</span>
              <input name="budgetCents" type="number" min="1" step="1" placeholder="4200" required />
            </label>
            <label className="auth-field">
              <span>Response deadline</span>
              <input name="responseDeadlineAt" type="datetime-local" required />
            </label>
          </div>
          <label className="auth-field">
            <span>Scope</span>
            <textarea name="scope" rows={3} placeholder="Investigate the failure, stabilize the runtime, and summarize next steps." required />
          </label>
          <button type="submit" className="auth-submit">
            Publish RFQ
          </button>
        </form>
      </article>

      <div className="feed-grid">
        <article className="feed-card">
          <span className="tag">Recommended listings</span>
          <h3>Providers ranked for the current market temperature.</h3>
          <div className="flex gap-2 mb-2 flex-wrap">
            <a href={buildListingCategoryHref("all")} className={chipClass(listingCategory === "all")} aria-current={listingCategory === "all" ? "page" : undefined}>
              All categories
            </a>
            {listingCategories.map((category) => (
              <a
                key={category}
                href={buildListingCategoryHref(category)}
                className={chipClass(listingCategory === category)}
                aria-current={listingCategory === category ? "page" : undefined}
              >
                {category}
              </a>
            ))}
          </div>
          <form method="GET" className="auth-form market-form">
            <input type="hidden" name="listingCategory" value={listingCategory} />
            <input type="hidden" name="rfqSearch" value={rfqSearch} />
            <input type="hidden" name="rfqStatusFilter" value={rfqStatusFilter} />
            <input type="hidden" name="messageSearch" value={messageSearch} />
            <label className="auth-field">
              <span>Search listings</span>
              <input
                name="listingSearch"
                type="text"
                placeholder="Search by title, category, or tag"
                defaultValue={listingSearch}
              />
            </label>
            <button type="submit" className="auth-submit">
              Filter listings
            </button>
          </form>
          <div className="feed-list">
            {filteredListings.length === 0 ? (
              <EmptyState
                icon="🔎"
                message="No live recommendations yet; open new RFQs to seed marketplace activity."
                actionLabel="Create RFQ now"
                actionHref="#create-rfq"
              />
            ) : null}
            {filteredListings.map((listing) => (
              <div key={listing.id} className="feed-item">
                <strong>{listing.title}</strong>
                <p>
                  {listing.category} · {formatMoney(listing.basePriceCents)} base price
                </p>
                <div className="chip-list">
                  {listing.tags.map((tag) => (
                    <div className="chip" key={tag}>
                      {tag}
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </article>

        <aside className="message-card" id="rfq-book">
          <span className="tag">RFQ book</span>
          <h3>Every open request should show bid pressure, not just status.</h3>
          <div className="flex gap-2 mb-2">
            <a href={buildRFQStatusHref("all")} className={chipClass(rfqStatusFilter === "all")} aria-current={rfqStatusFilter === "all" ? "page" : undefined}>
              All
            </a>
            <a href={buildRFQStatusHref("open")} className={chipClass(rfqStatusFilter === "open")} aria-current={rfqStatusFilter === "open" ? "page" : undefined}>
              Open
            </a>
            <a href={buildRFQStatusHref("awarded")} className={chipClass(rfqStatusFilter === "awarded")} aria-current={rfqStatusFilter === "awarded" ? "page" : undefined}>
              Awarded
            </a>
          </div>
          <form method="GET" className="auth-form market-form">
            <input type="hidden" name="listingCategory" value={listingCategory} />
            <input type="hidden" name="listingSearch" value={listingSearch} />
            <input type="hidden" name="messageSearch" value={messageSearch} />
            <div className="market-form__grid">
              <label className="auth-field">
                <span>Search RFQ book</span>
                <input
                  name="rfqSearch"
                  type="text"
                  placeholder="Search by title"
                  defaultValue={rfqSearch}
                />
              </label>
              <label className="auth-field">
                <span>Status</span>
                <select
                  name="rfqStatusFilter"
                  defaultValue={rfqStatusFilter}
                >
                  <option value="all">All</option>
                  <option value="open">Open</option>
                  <option value="awarded">Awarded</option>
                </select>
              </label>
            </div>
            <button type="submit" className="auth-submit">
              Filter RFQs
            </button>
          </form>
          <div className="message-list">
            {filteredRFQs.length === 0 ? (
              <EmptyState
                icon="🧾"
                message="No open RFQs to action. Create one above to start receiving bids."
                actionLabel="Clear filters"
                actionHref="/buyer"
              />
            ) : null}
            {filteredRFQs.map((rfq) => (
              <div key={rfq.id} className="message-item">
                <strong>{rfq.title}</strong>
                <p>
                  <StatusBadge status={rfq.status} /> · {rfq.bidCount} bids · budget {formatMoney(rfq.budgetCents)}
                </p>
                <p>Response deadline {rfq.responseDeadlineAt.slice(0, 10)}</p>
                <div className="message-list">
                  {rfq.bids.map((bid) => (
                    <form key={bid.id} className="inline-form" action={`/buyer/rfqs/${rfq.id}/award`} method="post">
                      <input type="hidden" name="bidId" value={bid.id} />
                      <input type="hidden" name="fundingMode" value="credit" />
                      <input type="hidden" name="creditLineId" value="credit_1" />
                      <div>
                        <strong>{bid.providerOrgId}</strong>
                        <p>
                          {bid.status} · quote {formatMoney(bid.quoteCents)}
                        </p>
                      </div>
                      <button type="submit" className="action-button">
                        Award
                      </button>
                    </form>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </aside>
      </div>

      <article className="feed-card" id="message-inbox">
        <span className="tag">Inbox</span>
        <h3>Messages that change buyer decisions.</h3>
        <form method="GET" className="auth-form market-form">
          <input type="hidden" name="listingCategory" value={listingCategory} />
          <input type="hidden" name="listingSearch" value={listingSearch} />
          <input type="hidden" name="rfqSearch" value={rfqSearch} />
          <input type="hidden" name="rfqStatusFilter" value={rfqStatusFilter} />
          <label className="auth-field">
            <span>Search messages</span>
            <input
              name="messageSearch"
              type="text"
              placeholder="Search inbox title or details"
              defaultValue={messageSearch}
            />
          </label>
          <button type="submit" className="auth-submit">
            Filter messages
          </button>
        </form>
        <div className="feed-list">
          {filteredMessages.length === 0 ? (
            <EmptyState
              icon="📭"
              message="No messages yet. You’re all clear for now; messages will appear here once bidders engage."
              actionLabel="Create RFQ now"
              actionHref="#create-rfq"
            />
          ) : null}
          {filteredMessages.map((message) => (
            <div key={message.id} className="feed-item">
              <strong>{message.title}</strong>
              <p>{message.detail}</p>
            </div>
          ))}
        </div>
      </article>

      <article className="timeline-card">
        <span className="tag">Active order frame</span>
        <h3>Milestone state is the thing that determines cash movement.</h3>
        <div className="timeline">
          {data.activeOrders[0]?.milestones.map((milestone) => (
            <div key={milestone.id} className="timeline-item">
              <strong>
                {milestone.title} · <StatusBadge status={milestone.state} />
              </strong>
              <p>
                Budget {formatMoney(milestone.budgetCents)} · Settled {formatMoney(milestone.settledCents)}
              </p>
              <ProgressBar
                current={milestone.settledCents}
                total={milestone.budgetCents}
                tone={milestone.settledCents > milestone.budgetCents * PROGRESS_WARNING_THRESHOLD ? "warning" : "default"}
              />
            </div>
          ))}
          {!data.activeOrders[0]?.milestones.length && (
            <EmptyState
              icon="📋"
              message="No active milestones yet."
              actionLabel="Create an RFQ"
              actionHref="#create-rfq"
            />
          )}
        </div>
      </article>
    </PortalShell>
  );
}


function readSearchParam(
  searchParams: Record<string, string | string[] | undefined> | undefined,
  key: string,
): string {
  const value = searchParams?.[key];
  return Array.isArray(value) ? value[0] ?? "" : value ?? "";
}
