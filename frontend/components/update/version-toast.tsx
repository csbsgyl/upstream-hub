"use client"

import { useEffect, useMemo, useRef } from "react"
import { toast } from "sonner"
import { useNavigate } from "react-router-dom"
import { useVersionCheck } from "@/lib/queries"
import { useAuth } from "@/lib/auth-context"

const SEEN_KEY = "uh_version_seen"

export function VersionToast() {
  const navigate = useNavigate()
  const { status } = useAuth()
  const version = useVersionCheck()
  const shownRef = useRef<string | null>(null)

  const id = useMemo(() => {
    const latest = version.data?.latest_commit ?? "unknown"
    return `${version.data?.current.commit ?? "unknown"}:${latest}`
  }, [version.data?.current.commit, version.data?.latest_commit])

  useEffect(() => {
    if (status !== "authenticated") return
    const data = version.data
    if (!data || data.check_error || !data.has_update) return
    if (shownRef.current === id) return
    if (typeof window !== "undefined" && window.localStorage.getItem(SEEN_KEY) === id) return

    shownRef.current = id
    if (typeof window !== "undefined") {
      window.localStorage.setItem(SEEN_KEY, id)
    }

    const latest = data.latest_short ?? "unknown"
    const current = data.current.short_commit
    toast.warning(`发现新版本 ${latest}`, {
      description: `当前 ${current}，设置里可以查看更新信息和部署命令。`,
      action: {
        label: "去设置",
        onClick: () => {
          navigate("/settings")
        },
      },
      cancel: {
        label: "打开 GitHub",
        onClick: () => {
          if (data.latest_html_url) {
            window.open(data.latest_html_url, "_blank", "noopener,noreferrer")
          }
        },
      },
      position: "bottom-right",
      duration: 8000,
    })
  }, [id, navigate, status, version.data])

  return null
}
