import { describe, expect, it } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type { BuyerDepositSummary } from "@1tok/contracts";

import { BuyerDepositPanel } from "@/components/buyer-deposit-panel";
import { buildBuyerDepositQRCodeURL, buildBuyerDepositQRCodeValue } from "./buyer-deposit";

const demoDeposit: BuyerDepositSummary = {
  buyerOrgId: "org_demo_buyer",
  asset: "USDI",
  address: "ckt1qzda0cr08m85hc8jlnfp3zer7xulejywt49kt2rr0vthywaa50xwsq0ep57pnjdssav7xf8vyqh0ml5ll7fww0sql4jqe",
  onChainBalance: "10.00",
  confirmedBalance: "0.00",
  creditedBalance: "50.00",
  creditedBalanceCents: 5000,
  minimumSweepAmount: "10.00",
  confirmationBlocks: 24,
};

describe("buyer deposit helpers", () => {
  it("builds a wallet-friendly QR payload and url", () => {
    expect(buildBuyerDepositQRCodeValue(demoDeposit.address)).toBe(demoDeposit.address);
    expect(buildBuyerDepositQRCodeURL(demoDeposit.address)).toContain(encodeURIComponent(demoDeposit.address));
  });

  it("renders QR and copy affordances for the buyer deposit address", () => {
    const html = renderToStaticMarkup(<BuyerDepositPanel deposit={demoDeposit} topUp="" />);

    expect(html).toContain("Copy address");
    expect(html).toContain("CKB deposit address QR code");
    expect(html).toContain(encodeURIComponent(demoDeposit.address));
    expect(html).toContain("Fixed CKB address for USDI funding.");
  });
});
