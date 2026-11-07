import { useState } from 'react'
import { FileText, ExternalLink, AlertTriangle } from 'lucide-react'
import { api } from '@/api/client'

// Links to a COA: the archived copy (presigned S3 URL, fetched on tap so it
// never expires on screen) and the original link. "Not archived" flags a COA
// whose download failed, so the record depends on the lab keeping the file up.
export function CoaLinks({ url, s3Key }: { url?: string; s3Key?: string }) {
  const [opening, setOpening] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!url && !s3Key) {
    return <span className="text-xs text-muted">No COA</span>
  }

  async function openArchived() {
    if (!s3Key) return
    // Open the tab synchronously (inside the tap) so mobile browsers don't
    // block it as a popup, then point it at the presigned URL.
    const tab = window.open('', '_blank')
    setOpening(true)
    setError(null)
    try {
      const { url: signed } = await api.media.getDownloadUrl(s3Key)
      if (tab) {
        tab.location.href = signed
      } else {
        window.location.href = signed
      }
    } catch (e) {
      tab?.close()
      setError(e instanceof Error ? e.message : 'Could not open COA')
    } finally {
      setOpening(false)
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
      {s3Key && (
        <button onClick={openArchived} disabled={opening} className="flex items-center gap-1 text-fern active:opacity-70 disabled:opacity-50">
          <FileText size={13} />
          {opening ? 'Opening…' : 'COA'}
        </button>
      )}
      {url && (
        <a href={url} target="_blank" rel="noreferrer" className="flex items-center gap-1 text-dim active:opacity-70">
          <ExternalLink size={13} />
          Original link
        </a>
      )}
      {url && !s3Key && (
        <span className="flex items-center gap-1 text-amber-400">
          <AlertTriangle size={13} />
          Not archived
        </span>
      )}
      {error && <span className="text-red-400">{error}</span>}
    </div>
  )
}
