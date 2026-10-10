import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import Link from "next/link";
import { LogIn } from "lucide-react";
import { Card } from "@/components/ui/card";
import { signupHref } from "@/lib/auth-routes";

export function isAuthErrorMessage(message: string): boolean {
  const lower = message.toLowerCase();
  return lower.includes("authentication") || lower.includes("로그인");
}

type LoginRequiredPanelProps = {
  loginNext?: string;
  description?: string;
};

export function LoginRequiredPanel({
  loginNext,
  description = "로그인 후 이용할 수 있습니다.",
}: LoginRequiredPanelProps) {
  return (
    <Card className="border-amber-500/30 bg-amber-500/5 p-6">
      <div className="flex gap-4">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-500/15 text-amber-300">
          <LogIn className="h-5 w-5" />
        </div>
        <div className="space-y-3 text-sm">
          <p className="text-base font-medium text-[var(--text-primary)]">
            로그인 후 이용할 수 있는 기능입니다
          </p>
          <p className="text-[var(--text-secondary)]">{description}</p>
          <div className="flex flex-wrap gap-2">
            <Link
              href={`/login${loginNext ? `?next=${encodeURIComponent(loginNext)}` : ""}`}
              className={cn(buttonVariants({ variant: "default", size: "default" }), "h-9 bg-[var(--brand)] text-white hover:opacity-90")}
            >
              로그인
            </Link>
            <Link
              href={signupHref(loginNext)}
              className={cn(buttonVariants({ variant: "outline", size: "lg" }), "border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]")}
            >
              회원가입
            </Link>
          </div>
        </div>
      </div>
    </Card>
  );
}
