import { useState, useEffect, useRef } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ImagePlus, X } from 'lucide-react'
import { BottomSheet } from './BottomSheet'
import { api } from '@/api/client'
import type { Environment, EnvironmentType, CreateEnvironmentRequest } from '@/types'

const TYPES: EnvironmentType[] = ['tent', 'outdoor', 'garage', 'basement', 'room', 'greenhouse', 'other']

const inputCls = 'w-full bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary placeholder:text-muted focus:outline-none focus:border-fern'
const labelCls = 'block text-xs text-dim mb-1'

const emptyForm: CreateEnvironmentRequest = { name: '', type: 'tent' }

interface Props {
  open: boolean
  onClose: () => void
  environment?: Environment
}

export function AddEnvironmentSheet({ open, onClose, environment }: Props) {
  const qc = useQueryClient()
  const isEdit = !!environment
  const fileRef = useRef<HTMLInputElement>(null)

  const [form, setForm] = useState<CreateEnvironmentRequest>(emptyForm)
  const [photoFile, setPhotoFile] = useState<File | null>(null)
  const [photoPreview, setPhotoPreview] = useState<string | null>(null)

  // Pre-signed URL for existing photo (edit mode)
  const { data: existingPhotoData } = useQuery({
    queryKey: ['media-url', environment?.photoKey],
    queryFn: () => api.media.getDownloadUrl(environment!.photoKey!),
    enabled: open && !!environment?.photoKey,
    staleTime: 50 * 60 * 1000,
  })

  useEffect(() => {
    if (open) {
      setPhotoFile(null)
      setPhotoPreview(null)
      setForm(environment
        ? {
            name:           environment.name,
            type:           environment.type,
            lightSchedule:  environment.lightSchedule,
            targetTempF:    environment.targetTempF || undefined,
            targetHumidity: environment.targetHumidity || undefined,
            photoKey:       environment.photoKey,
          }
        : emptyForm)
    }
  }, [open, environment])

  // Clean up local object URL
  useEffect(() => {
    return () => { if (photoPreview) URL.revokeObjectURL(photoPreview) }
  }, [photoPreview])

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    if (photoPreview) URL.revokeObjectURL(photoPreview)
    setPhotoFile(file)
    setPhotoPreview(URL.createObjectURL(file))
  }

  function clearPhoto() {
    if (photoPreview) URL.revokeObjectURL(photoPreview)
    setPhotoFile(null)
    setPhotoPreview(null)
    setForm(f => ({ ...f, photoKey: undefined }))
    if (fileRef.current) fileRef.current.value = ''
  }

  const mutation = useMutation({
    mutationFn: async (body: CreateEnvironmentRequest) => {
      if (photoFile) {
        body.photoKey = await api.media.uploadFile(photoFile, 'environments/covers')
      }
      return isEdit
        ? api.environments.update(environment!.environmentId, body)
        : api.environments.create(body)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['environments'] })
      onClose()
    },
  })

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    mutation.mutate({ ...form })
  }

  const displayPhoto = photoPreview ?? existingPhotoData?.url ?? null

  return (
    <BottomSheet title={isEdit ? 'Edit Environment' : 'New Environment'} open={open} onClose={onClose}>
      <form onSubmit={submit} className="space-y-3 mt-2">

        {/* Photo picker */}
        <div>
          <label className={labelCls}>Photo</label>
          <div className="relative">
            {displayPhoto ? (
              <div className="relative w-full h-36 rounded-xl overflow-hidden border border-border">
                <img src={displayPhoto} alt="Environment" className="w-full h-full object-cover" />
                <button
                  type="button"
                  onClick={clearPhoto}
                  className="absolute top-2 right-2 w-7 h-7 bg-black/60 rounded-full flex items-center justify-center text-white active:opacity-70"
                >
                  <X size={14} />
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => fileRef.current?.click()}
                className="w-full h-24 rounded-xl border border-dashed border-border flex flex-col items-center justify-center gap-1.5 text-muted active:opacity-70"
              >
                <ImagePlus size={22} />
                <span className="text-xs">Add photo</span>
              </button>
            )}
            {displayPhoto && (
              <button
                type="button"
                onClick={() => fileRef.current?.click()}
                className="mt-1.5 text-xs text-fern active:opacity-70"
              >
                Change photo
              </button>
            )}
            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={onFileChange}
            />
          </div>
        </div>

        <div>
          <label className={labelCls}>Name *</label>
          <input
            className={inputCls}
            placeholder="e.g. 4x4 Tent"
            value={form.name}
            onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            required
          />
        </div>

        <div>
          <label className={labelCls}>Type</label>
          <select
            className={inputCls}
            value={form.type}
            onChange={e => setForm(f => ({ ...f, type: e.target.value as EnvironmentType }))}
          >
            {TYPES.map(t => (
              <option key={t} value={t} className="bg-raised capitalize">{t}</option>
            ))}
          </select>
        </div>

        <div>
          <label className={labelCls}>Light Schedule</label>
          <input
            className={inputCls}
            placeholder="e.g. 18/6"
            value={form.lightSchedule ?? ''}
            onChange={e => setForm(f => ({ ...f, lightSchedule: e.target.value || undefined }))}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>Target Temp (°F)</label>
            <input
              type="number"
              className={inputCls}
              placeholder="75"
              value={form.targetTempF ?? ''}
              onChange={e => setForm(f => ({ ...f, targetTempF: e.target.value ? Number(e.target.value) : undefined }))}
            />
          </div>
          <div>
            <label className={labelCls}>Target Humidity (%)</label>
            <input
              type="number"
              className={inputCls}
              placeholder="55"
              value={form.targetHumidity ?? ''}
              onChange={e => setForm(f => ({ ...f, targetHumidity: e.target.value ? Number(e.target.value) : undefined }))}
            />
          </div>
        </div>

        {mutation.isError && (
          <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>
        )}

        <button
          type="submit"
          disabled={mutation.isPending}
          className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50 mt-2"
        >
          {mutation.isPending ? 'Saving…' : isEdit ? 'Save Changes' : 'Add Environment'}
        </button>
      </form>
    </BottomSheet>
  )
}
