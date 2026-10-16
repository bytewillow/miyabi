import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiGet, apiPut } from '@/api/client'

export type StrmConfig = {
  public_url: string
  effective_url: string
  rewritten?: number
}

export const strmKeys = {
  config: ['settings', 'strm'] as const
}

export function useStrmConfig() {
  return useQuery({
    queryKey: strmKeys.config,
    queryFn: ({ signal }) => apiGet<StrmConfig>('/api/settings/strm', undefined, signal),
    staleTime: 15_000
  })
}

export function useUpdateStrmConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (publicUrl: string) =>
      apiPut<StrmConfig>('/api/settings/strm', { public_url: publicUrl }),
    onSuccess: next => {
      queryClient.setQueryData(strmKeys.config, next)
    }
  })
}
