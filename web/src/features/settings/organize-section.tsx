import { useState } from 'react'
import { FolderSyncIcon, LoaderCircleIcon } from 'lucide-react'
import { toast } from 'sonner'

import { describeApiError } from '@/api/client'
import { useOrganizeConfig, useUpdateOrganizeConfig, type OrganizeMode } from '@/api/organize'
import { InlineError } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { SettingRow, SettingsSection } from './shared'

export function OrganizeSection() {
  const organize = useOrganizeConfig()
  const updateConfig = useUpdateOrganizeConfig()

  const saved: OrganizeMode = organize.data?.mode ?? 'export'
  const [form, setForm] = useState<OrganizeMode | null>(null)
  const current = form ?? saved
  const isDirty = form !== null && current !== saved

  if (form !== null && !isDirty && !updateConfig.isPending) setForm(null)

  const disabled = organize.isLoading || organize.isError || updateConfig.isPending

  function handleSave() {
    updateConfig.mutate(current, {
      onSuccess: next => {
        setForm(next.mode)
        toast.success('媒体组织方式已保存', {
          description:
            next.mode === 'pan'
              ? '之后的刮削会把媒体文件在 115 内按番号整理，并上传 NFO 与海报'
              : '之后的刮削会生成本地 STRM 导出目录'
        })
      },
      onError: error => {
        toast.error(describeApiError(error))
      }
    })
  }

  return (
    <SettingsSection icon={<FolderSyncIcon className="size-4" />} title="媒体组织">
      {organize.isError ? <InlineError>后端服务暂不可用，无法读取媒体组织配置。</InlineError> : null}
      <SettingRow
        title="组织方式"
        description="STRM 导出：在本地生成 .strm 与 NFO 目录供 Emby / Jellyfin 读取，115 上的文件不动。115 原地整理：刮削后直接在 115 资料库内按 <前缀>/<番号>/ 移动并重命名真实文件，NFO 与海报一并上传，播放器挂载网盘即可直读；已整理的内容不会随切换自动迁移"
      >
        <Select
          value={current}
          disabled={disabled}
          onValueChange={value => setForm(value as OrganizeMode)}
        >
          <SelectTrigger className="w-full sm:w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="export">STRM 导出（默认）</SelectItem>
            <SelectItem value="pan">115 原地整理</SelectItem>
          </SelectContent>
        </Select>
      </SettingRow>

      <div className="flex items-center justify-end gap-2">
        <Button type="button" size="sm" disabled={disabled || !isDirty} onClick={handleSave}>
          {updateConfig.isPending ? <LoaderCircleIcon className="animate-spin" /> : null}
          保存
        </Button>
      </div>
    </SettingsSection>
  )
}
