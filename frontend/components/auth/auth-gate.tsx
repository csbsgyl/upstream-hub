"use client"

import { useAuth } from "@/lib/auth-context"
import { LoginPage } from "@/components/auth/login-page"
import { ChangePasswordPage } from "@/components/auth/change-password-page"
import type { ReactNode } from "react"

/**
 * AuthGate 把根渲染分成四态：
 *   loading                  本地有 token 但还没验完 — 显示占位
 *   anonymous                未登录 — 显示登录页
 *   authenticated + 需改密    已登录但首次需改密 — 显示强制改密页
 *   authenticated            已登录 — 显示业务内容
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { status, mustChangePassword } = useAuth()

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
        加载中…
      </div>
    )
  }
  if (status === "anonymous") {
    return <LoginPage />
  }
  if (mustChangePassword) {
    return <ChangePasswordPage />
  }
  return <>{children}</>
}
