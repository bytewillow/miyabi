import { useMemo, useState } from 'react'
import { ClapperboardIcon, LoaderCircleIcon, RefreshCwIcon } from 'lucide-react'
import { toast } from 'sonner'

import { describeApiError } from '@/api/client'
import { type PlexConfig, usePlexConfig, useTestPlexConfig, useUpdatePlexConfig } from '@/api/plex'
import { InlineError } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SettingRow, SettingsSection } from './shared'

interface PlexFormData {
  enabled: boolean
  server_url: string
  token: string
  media_path: string
}

function configToFormData(config?: PlexConfig | null): PlexFormData {
  return {
    enabled: config?.enabled ?? false,
    server_url: config?.server_url ?? '',
    token: config?.token ?? '',
    media_path: config?.media_path ?? ''
  }
}

export function PlexSection() {
  const plex = usePlexConfig()
  const updateConfig = useUpdatePlexConfig()
  const testConfig = useTestPlexConfig()

  const config = plex.data
  const initial = useMemo(() => configToFormData(config), [config])
  const [form, setForm] = useState<PlexFormData | null>(null)

  const current = form ?? initial

  const isDirty =
    Boolean(config) &&
    form !== null &&
    (current.enabled !== initial.enabled ||
      current.server_url !== initial.server_url ||
      current.token !== initial.token ||
      current.media_path !== initial.media_path)

  // Keep the saved form visible until the query observer receives the same
  // values. Clearing it in the success callback can briefly reveal old data.
  if (form !== null && !isDirty && !updateConfig.isPending) setForm(null)

  const disabled = plex.isLoading || plex.isError || updateConfig.isPending

  function updateField<K extends keyof PlexFormData>(key: K, value: PlexFormData[K]) {
    setForm(prev => ({
      ...(prev ?? initial),
      [key]: value
    }))
  }

  function handleToggle(checked: boolean) {
    if (checked) {
      updateField('enabled', true)
      return
    }

    setForm(null)
    if (!config?.enabled) return

    updateConfig.mutate(
      { ...config, enabled: false },
      {
        onSuccess: next => {
          setForm(configToFormData(next))
          toast.success('已关闭 Plex')
        },
        onError: error => {
          toast.error(describeApiError(error))
        }
      }
    )
  }

  function handleTest() {
    const serverUrl = current.server_url.trim()
    const token = current.token.trim()

    if (!serverUrl) {
      toast.error('请填写 Plex 服务器地址')
      return
    }
    if (!token) {
      toast.error('请填写 Plex Token')
      return
    }

    testConfig.mutate(
      { server_url: serverUrl, token },
      {
        onSuccess: data => {
          toast.success('Plex 连接成功', {
            description: `${data.server_name} (v${data.version})`
          })
        },
        onError: error => {
          toast.error(describeApiError(error))
        }
      }
    )
  }

  function handleSave() {
    const serverUrl = current.server_url.trim()
    const token = current.token.trim()

    if (current.enabled) {
      if (!serverUrl) {
        toast.error('启用 Plex 集成时必须填写服务器地址')
        return
      }
      if (!token) {
        toast.error('启用 Plex 集成时必须填写 Plex Token')
        return
      }
    }

    const payload: PlexConfig = {
      enabled: current.enabled,
      server_url: serverUrl,
      token,
      media_path: current.media_path.trim()
    }

    updateConfig.mutate(payload, {
      onSuccess: next => {
        setForm(configToFormData(next))
        toast.success('Plex 设置已保存')
      },
      onError: error => {
        toast.error(describeApiError(error))
      }
    })
  }

  return (
    <SettingsSection icon={<ClapperboardIcon className="size-4" />} title="Plex">
      <SettingRow
        title="启用 Plex"
        description="媒体扫描与刮削完成后，主动通知 Plex 对对应资料库目录进行局部扫描入库"
        inline
      >
        <Switch checked={current.enabled} disabled={disabled} onCheckedChange={handleToggle} />
      </SettingRow>

      {current.enabled ? (
        <>
          <SettingRow title="服务器地址" description="Plex 服务的完整地址，含端口，默认 32400">
            <Input
              autoFocus={form?.enabled === true && !initial.enabled}
              value={current.server_url}
              placeholder="http://192.168.1.100:32400"
              disabled={disabled}
              onChange={e => updateField('server_url', e.target.value)}
            />
          </SettingRow>

          <SettingRow
            title="X-Plex-Token"
            description="在 Plex Web 任一媒体页「查看 XML」地址或账号设置中获取"
          >
            <Input
              type="password"
              value={current.token}
              placeholder="填入 X-Plex-Token"
              disabled={disabled}
              onChange={e => updateField('token', e.target.value)}
            />
          </SettingRow>

          <SettingRow
            title="Plex 媒体库路径"
            description="Plex 容器内挂载的对应目录。留空则默认使用本地导出路径"
          >
            <Input
              value={current.media_path}
              placeholder="/media"
              disabled={disabled}
              onChange={e => updateField('media_path', e.target.value)}
            />
          </SettingRow>

          <div className="flex items-center justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled || testConfig.isPending}
              onClick={handleTest}
            >
              <RefreshCwIcon className={testConfig.isPending ? 'animate-spin' : undefined} />
              测试连接
            </Button>
            <Button type="button" size="sm" disabled={disabled || !isDirty} onClick={handleSave}>
              {updateConfig.isPending ? <LoaderCircleIcon className="animate-spin" /> : null}
              保存
            </Button>
          </div>
        </>
      ) : null}

      {plex.isError ? <InlineError>后端服务暂不可用，无法读取 Plex 配置。</InlineError> : null}
    </SettingsSection>
  )
}
