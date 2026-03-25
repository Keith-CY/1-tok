import type { BuyerDepositSummary } from "@1tok/contracts";
import { RiArrowDownSLine, RiQrCodeLine } from "react-icons/ri";

import { CopyDepositAddressButton } from "@/components/copy-deposit-address-button";
import { Card } from "@/components/ui/card";
import { buildBuyerDepositQRCodeURL } from "@/lib/buyer-deposit";

export function BuyerDepositPanel({
  deposit,
  topUp,
}: {
  deposit: BuyerDepositSummary | null;
  topUp: string;
}) {
  const sweepRule = deposit ? `Min ${deposit.minimumSweepAmount} USDI / ${deposit.confirmationBlocks} blocks` : "";

  return (
    <div className="space-y-4">
      {deposit ? (
        <details className="group market-card overflow-hidden">
          <summary className="list-none cursor-pointer px-5 py-5 [&::-webkit-details-marker]:hidden">
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-2">
                <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">Deposit address</div>
                <div className="font-display text-[clamp(1.15rem,1.7vw,1.45rem)] font-medium leading-[1] tracking-[-0.03em] text-balance">
                  USDI Funding Address (CKB)
                </div>
              </div>
              <span className="sheet-stack flex size-11 shrink-0 items-center justify-center bg-card transition-transform duration-200 group-open:rotate-180">
                <RiArrowDownSLine className="size-5 text-foreground" />
              </span>
            </div>
          </summary>

          <div className="space-y-4 bg-background px-5 pb-5">
            <div className="bg-card px-4 py-4 shadow-[0_20px_40px_rgba(0,0,0,0.06)]">
              <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">
                <RiQrCodeLine className="size-4 text-accent" />
                QR code
              </div>
              <div className="mt-3 flex w-full justify-center overflow-hidden bg-white p-2">
                <img
                  src={buildBuyerDepositQRCodeURL(deposit.address)}
                  alt="CKB deposit address QR code"
                  width={256}
                  height={256}
                  className="w-full max-w-[16rem]"
                />
              </div>
            </div>

            <div className="space-y-3">
              <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">CKB address</div>
              <div className="bg-card px-4 py-4 font-mono text-sm leading-7 break-all shadow-[0_20px_40px_rgba(0,0,0,0.06)]">{deposit.address}</div>
              <CopyDepositAddressButton address={deposit.address} />
            </div>

            <div className="sheet-stack bg-card px-4 py-4 text-xs leading-6 text-muted-foreground shadow-[0_20px_40px_rgba(0,0,0,0.04)]">
              <div className="flex flex-wrap items-baseline gap-x-6 gap-y-2">
                <div className="space-x-2">
                  <span className="text-[10px] font-semibold uppercase tracking-[0.22em]">On-chain USDI</span>
                  <span className="font-mono text-sm font-semibold text-foreground">{deposit.onChainBalance}</span>
                </div>
                <div className="space-x-2">
                  <span className="text-[10px] font-semibold uppercase tracking-[0.22em]">Sweep rule</span>
                  <span className="font-mono text-sm font-semibold text-foreground">{sweepRule}</span>
                </div>
              </div>
            </div>
          </div>
        </details>
      ) : (
        <Card className="bg-card p-6 text-sm leading-7 text-muted-foreground">Settlement has not exposed a buyer deposit address yet.</Card>
      )}
      {topUp === "failed" ? (
        <div className="max-w-2xl text-sm leading-6 text-muted-foreground">
          Deposit address lookup failed. Try again after checking settlement connectivity.
        </div>
      ) : null}
    </div>
  );
}
