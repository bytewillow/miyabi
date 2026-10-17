import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiGet, apiPut } from '@/api/client'

export type OrganizeMode = 'export' | 'pan'

export type OrganizeConfig = {
  mode: OrganizeMode
}

export const organizeKeys = {
  config: ['settings', 'organize'] as const
}

export function useOrganizeConfig() {
  return useQuery({
    queryKey: organizeKeys.config,
    queryFn: ({ signal }) => apiGet<OrganizeConfig>('/api/settings/organize', undefined, signal),
    staleTime: 15_000
  })
}

export function useUpdateOrganizeConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (mode: OrganizeMode) => apiPut<OrganizeConfig>('/api/settings/organize', { mode }),
    onSuccess: next => {
      queryClient.setQueryData(organizeKeys.config, next)
    }
  })
}
