"use client"

import { useState, type FormEvent } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/lib/auth-context"
import type { ApiError } from "@/lib/api"

const MIN_PASSWORD_LEN = 6

/**
 * ChangePasswordPage 在"首次登录强制改密"场景下渲染。
 * 默认账号 admin/admin 登录后 mustChangePassword=true，必须先改密才能进入面板。
 */
export function ChangePasswordPage() {
  const { changePassword, logout, username } = useAuth()
  const [oldPassword, setOldPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    if (newPassword.length < MIN_PASSWORD_LEN) {
      setError(`新密码至少 ${MIN_PASSWORD_LEN} 位`)
      return
    }
    if (newPassword !== confirm) {
      setError("两次输入的新密码不一致")
      return
    }
    if (newPassword === oldPassword) {
      setError("新密码不能与当前密码相同")
      return
    }
    setSubmitting(true)
    try {
      await changePassword(oldPassword, newPassword)
    } catch (err) {
      const e = err as ApiError
      setError(e.message || "修改失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1.5">
          <CardTitle className="text-2xl">修改密码</CardTitle>
          <CardDescription>
            首次登录{username ? `（${username}）` : ""}需要修改默认密码后才能继续使用。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="old-password">当前密码</Label>
              <Input
                id="old-password"
                type="password"
                autoComplete="current-password"
                value={oldPassword}
                onChange={(e) => setOldPassword(e.target.value)}
                required
                disabled={submitting}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">新密码</Label>
              <Input
                id="new-password"
                type="password"
                autoComplete="new-password"
                placeholder={`至少 ${MIN_PASSWORD_LEN} 位`}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                disabled={submitting}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password">确认新密码</Label>
              <Input
                id="confirm-password"
                type="password"
                autoComplete="new-password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                required
                disabled={submitting}
              />
            </div>
            {error ? (
              <p className="text-sm text-destructive" role="alert">
                {error}
              </p>
            ) : null}
            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting ? "提交中…" : "修改并进入"}
            </Button>
            <Button
              type="button"
              variant="ghost"
              className="w-full"
              onClick={logout}
              disabled={submitting}
            >
              退出登录
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
