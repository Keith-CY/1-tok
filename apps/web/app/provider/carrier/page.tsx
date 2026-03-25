import { redirect } from "next/navigation";

export const metadata = {
  title: "Provider Carrier",
};

export const dynamic = "force-dynamic";

export default async function ProviderCarrierPage() {
  redirect("/provider");
}
