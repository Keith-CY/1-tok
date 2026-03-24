import { redirect } from "next/navigation";

export const metadata = {
  title: "New Listing",
};

export const dynamic = "force-dynamic";

export default async function ProviderListingsCreatePage() {
  redirect("/provider");
}
