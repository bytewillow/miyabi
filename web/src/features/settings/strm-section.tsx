import { useState } from 'react'
import { Link2Icon, LoaderCircleIcon } from 'lucide-react'
import { toast } from 'sonner'

import { describeApiError } from '@/api/client'
import { useStrmConfig, useUpdateStrmConfig } from '@/api/strm'
import { InlineError } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { SettingRow, SettingsSection } from './shared'

export function StrmSection() {
  const strm = useStrmConfig()
  const updateConfig = useUpdateStrmConfig()

  const saved = strm.data?.public_url ?? ''
  const effective = strm.data?.effective_url ?? ''
  const [form, setForm] = useState<string | null>(null)
  const current = form ?? saved
  const isDirty = form !== null && current !== saved

  // Same pattern as the Plex section: keep the saved form visible until the
  // query cache reflects it.
  if (form !== null && !isDirty && !updateConfig.isPending) setForm(null)

  const disabled = strm.isLoading || strm.isError || updateConfig.isPending

  function handleSave() {
    updateConfig.mutate(current.trim(), {
      onSuccess: next => {
        setForm(next.public_url)
        toast.success('STRM 播放地址已保存', {
          description: `已重写 ${next.rewritten ?? 0} 个 STRM 文件，当前地址 ${next.effective_url}`
        })
      },
      onError: error => {
        toast.error(describeApiError(error))
      }
    })
  }

  return (
    <SettingsSection icon={<Link2Icon className="size-4" />} title="STRM 播放">
      <SettingRow
        title="播放地址"
        description="写入 .strm 文件的 Miyabi 地址，Emby / Plex 等播放器按此地址回连取流，播放器必须能访问。留空则使用环境变量 MIYABI_PUBLIC_URL 或自动探测地址；保存后自动重写全部 STRM 文件"
      >
        <Input
          value={current}
          placeholder={effective || 'http://192.168.1.10:8088'}
          disabled={disabled}
          onChange={e => setForm(e.target.value)}
        />
      </SettingRow>

      <div className="flex items-center justify-end gap-2">
        <Button type="button" size="sm" disabled={disabled || !isDirty} onClick={handleSave}>
          {updateConfig.isPending ? <LoaderCircleIcon className="animate-spin" /> : null}
          保存
        </Button>
      </div>

      {strm.isError ? <InlineError>后端服务暂不可用，无法读取 STRM 配置。</InlineError> : null}
    </SettingsSection>
  )
}
