import {
  mutationOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient
} from '@tanstack/react-query'

import { apiGet, apiPost, apiPut } from '@/api/client'

export type PlexConfig = {
  enabled: boolean
  server_url: string
  token: string
  media_path: string
  local_dir?: string
}

export type PlexServerInfo = {
  server_name: string
  version: string
  id: string
}

export const plexKeys = {
  config: ['settings', 'plex'] as const
}

export function usePlexConfig() {
  return useQuery({
    queryKey: plexKeys.config,
    queryFn: ({ signal }) => apiGet<PlexConfig>('/api/settings/plex', undefined, signal),
    staleTime: 15_000
  })
}

export function updatePlexConfigOptions(queryClient: QueryClient) {
  return mutationOptions({
    mutationFn: (config: PlexConfig) => apiPut<PlexConfig>('/api/settings/plex', config),
    onMutate: async next => {
      await queryClient.cancelQueries({ queryKey: plexKeys.config })
      const previous = queryClient.getQueryData<PlexConfig>(plexKeys.config)
      queryClient.setQueryData<PlexConfig>(plexKeys.config, {
        ...previous,
        ...next,
        local_dir: next.local_dir || previous?.local_dir
      })
      return { previous }
    },
    onSuccess: next => {
      queryClient.setQueryData(plexKeys.config, next)
    },
    onError: (_error, _next, context) => {
      if (context?.previous) queryClient.setQueryData(plexKeys.config, context.previous)
      void queryClient.invalidateQueries({ queryKey: plexKeys.config })
    }
  })
}

export function useUpdatePlexConfig() {
  return useMutation(updatePlexConfigOptions(useQueryClient()))
}

export function useTestPlexConfig() {
  return useMutation({
    mutationFn: (config?: Partial<PlexConfig>) =>
      apiPost<PlexServerInfo>('/api/settings/plex/test', config)
  })
}
