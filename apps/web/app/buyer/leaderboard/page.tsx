import { redirect } from "next/navigation";

export const metadata = {
  title: "Buyer Leaderboard",
};

export const dynamic = "force-dynamic";

export default async function LeaderboardPage() {
  redirect("/buyer");
}
