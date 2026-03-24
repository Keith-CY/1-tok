import { redirect } from "next/navigation";

export const metadata = {
  title: "Provider Listings",
};

export const dynamic = "force-dynamic";

export default async function ProviderListingsPage() {
  redirect("/provider");
}
