import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

interface Props {
  photoKey: string
  alt: string
  className?: string
  fallback?: React.ReactNode
}

export function MediaImage({ photoKey, alt, className, fallback = null }: Props) {
  const { data } = useQuery({
    queryKey: ['media-url', photoKey],
    queryFn: () => api.media.getDownloadUrl(photoKey),
    staleTime: 50 * 60 * 1000,
  })
  if (!data?.url) return <>{fallback}</>
  return <img src={data.url} alt={alt} className={className} />
}
