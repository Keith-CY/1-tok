import { PublicShell } from "@/components/workspace-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

export const metadata = {
  title: "Ops Sign In",
};

export const dynamic = "force-dynamic";

const errorMessages: Record<string, string> = {
  "invalid-credentials": "账号或密码不正确。",
  "missing-fields": "请填写邮箱和密码。",
};

export default async function InternalLoginPage({
  searchParams,
}: {
  searchParams?: Promise<{ error?: string; next?: string }>;
}) {
  const params = await searchParams;
  const error = params?.error ? errorMessages[params.error] ?? "登录失败，请稍后再试。" : null;
  const next = params?.next?.startsWith("/ops") ? params.next : "/ops";

  return (
    <PublicShell mode="internal">
      <div className="mx-auto grid flex-1 w-full max-w-4xl gap-6 lg:grid-cols-[1fr_0.9fr] lg:items-start">
        <div className="space-y-4">
          <h1 className="font-display text-4xl text-balance">内部处理入口</h1>
          <p className="max-w-2xl text-sm leading-7 text-muted-foreground text-pretty">这个入口仅用于内部运营处理审核、争议和人工决策，不对外展示。</p>
        </div>
        <Card>
          <CardHeader className="border-b border-border/70">
            <CardTitle>内部登录</CardTitle>
            <CardDescription>登录后直接进入运营处理台。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 p-5">
            <form action="/auth/login" method="post" className="space-y-4">
              <input type="hidden" name="next" value={next} />
              <label className="grid gap-2">
                <span className="text-sm font-medium text-foreground">邮箱</span>
                <Input name="email" type="email" autoComplete="email" placeholder="ops@example.com" required />
              </label>
              <label className="grid gap-2">
                <span className="text-sm font-medium text-foreground">密码</span>
                <Input name="password" type="password" autoComplete="current-password" placeholder="请输入密码" required />
              </label>
              {error ? <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</div> : null}
              <Button type="submit" className="w-full">进入处理台</Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </PublicShell>
  );
}
