import type { BuyerDepositSummary } from "@1tok/contracts";
import { RiExternalLinkLine, RiQrCodeLine } from "react-icons/ri";

import { CopyDepositAddressButton } from "@/components/copy-deposit-address-button";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { buildBuyerDepositQRCodeURL } from "@/lib/buyer-deposit";

export function BuyerDepositPanel({
  deposit,
  topUp,
}: {
  deposit: BuyerDepositSummary | null;
  topUp: string;
}) {
  return (
    <div className="space-y-4">
      {deposit ? (
        <div className="grid gap-4 lg:grid-cols-[12rem_1fr]">
          <div className="rounded-md border border-border bg-background/70 p-3">
            <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
              <RiQrCodeLine className="size-4 text-primary" />
              QR
            </div>
            <div className="mt-3 overflow-hidden rounded-md border border-border bg-white p-2">
              <img
                src={buildBuyerDepositQRCodeURL(deposit.address)}
                alt="CKB deposit address QR code"
                width={192}
                height={192}
                className="size-full"
              />
            </div>
          </div>

          <div className="space-y-4">
            <div className="rounded-md border border-border bg-background px-3 py-3 font-mono text-sm break-all">{deposit.address}</div>
            <div className="flex flex-wrap gap-2">
              <CopyDepositAddressButton address={deposit.address} />
              <Button asChild type="button" variant="outline" size="sm">
                <a href={buildBuyerDepositQRCodeURL(deposit.address)} target="_blank" rel="noreferrer">
                  Open QR
                  <RiExternalLinkLine className="size-4" />
                </a>
              </Button>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <SpotMetric label="Address USDI" value={deposit.onChainBalance} />
              <SpotMetric label="Confirmed USDI" value={deposit.confirmedBalance} />
              <SpotMetric label="Credited USD" value={deposit.creditedBalance} />
              <SpotMetric label="Sweep threshold" value={`${deposit.minimumSweepAmount} / ${deposit.confirmationBlocks} blocks`} />
            </div>
          </div>
        </div>
      ) : (
        <Card className="border-dashed p-6 text-sm text-muted-foreground">Settlement has not exposed a buyer deposit address yet.</Card>
      )}
      <div className="text-sm text-muted-foreground">
        {topUp === "success"
          ? "Deposit address refreshed."
          : topUp === "failed"
            ? "Deposit address lookup failed. Try again after checking settlement connectivity."
            : "Transfer USDI to the address above. Credits only finalize after the deposit is confirmed and swept into treasury."}
      </div>
    </div>
  );
}

function SpotMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-secondary/50 px-4 py-4">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{label}</div>
      <div className="mt-2 font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">{value}</div>
    </div>
  );
}
