import type { BuyerDepositSummary } from "@1tok/contracts";
import { RiArrowDownSLine, RiQrCodeLine } from "react-icons/ri";

import { CopyDepositAddressButton } from "@/components/copy-deposit-address-button";
import { Card } from "@/components/ui/card";
import { buildBuyerDepositQRCodeURL } from "@/lib/buyer-deposit";

const centsToAmount = (cents: number): string => {
  const sign = cents < 0 ? "-" : "";
  const absolute = Math.abs(cents);
  return `${sign}${Math.floor(absolute / 100)}.${String(absolute % 100).padStart(2, "0")}`;
};

const parseAmountCents = (value: string | undefined | null): number => {
  if (typeof value !== "string") {
    return 0;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return 0;
  }
  const normalized = trimmed.replace(/^\+/, "");
  const [whole, fractional = ""] = normalized.split(".");
  const fraction = (fractional + "00").slice(0, 2);
  const cents = (Number.parseInt(whole || "0", 10) || 0) * 100 + (Number.parseInt(fraction, 10) || 0);
  return cents;
};

const inferRawUnitsPerWholeUSDI = (deposit: BuyerDepositSummary): number => {
  if (deposit.rawMinimumSweepUnits && deposit.minimumSweepAmount) {
    const minimumCents = parseAmountCents(deposit.minimumSweepAmount);
    if (minimumCents > 0) {
      return Math.round((deposit.rawMinimumSweepUnits * 100) / minimumCents);
    }
  }
  return 100_000_000;
};

const normalizeOnChainBalance = (deposit: BuyerDepositSummary): string => {
  const rawUnits = deposit.rawOnChainUnits;
  if (typeof rawUnits !== "number" || rawUnits <= 0) {
    return deposit.onChainBalance;
  }
  const rawUnitsPerWholeUSDI = inferRawUnitsPerWholeUSDI(deposit);
  if (rawUnitsPerWholeUSDI <= 0) {
    return deposit.onChainBalance;
  }
  const displayCents = parseAmountCents(deposit.onChainBalance);
  const rawCents = Math.round((rawUnits * 100) / rawUnitsPerWholeUSDI);
  if (displayCents > 0 && (rawCents > displayCents * 10 || rawCents < displayCents / 10)) {
    return centsToAmount(rawCents);
  }
  return deposit.onChainBalance;
};

export function BuyerDepositPanel({
  deposit,
  topUp,
}: {
  deposit: BuyerDepositSummary | null;
  topUp: string;
}) {
  const sweepRule = deposit ? `Min ${deposit.minimumSweepAmount} USDI / ${deposit.confirmationBlocks} blocks` : "";
  const onChainBalance = deposit ? normalizeOnChainBalance(deposit) : "";

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
                  <span className="font-mono text-sm font-semibold text-foreground">{onChainBalance}</span>
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
