const buyerDepositQRCodeBaseURL = "https://api.qrserver.com/v1/create-qr-code/";

export function buildBuyerDepositQRCodeValue(address: string): string {
  return `ckb:${address.trim()}`;
}

export function buildBuyerDepositQRCodeURL(address: string, size = 192): string {
  const params = new URLSearchParams({
    size: `${size}x${size}`,
    data: buildBuyerDepositQRCodeValue(address),
  });
  return `${buyerDepositQRCodeBaseURL}?${params.toString()}`;
}
