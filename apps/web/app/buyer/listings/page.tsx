import { redirect } from "next/navigation";

export const metadata = {
  title: "Buyer Listings",
};

export const dynamic = "force-dynamic";

export default async function BuyerListingsPage() {
  redirect("/buyer");
}
