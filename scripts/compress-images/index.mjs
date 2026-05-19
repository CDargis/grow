import {
  S3Client,
  ListObjectsV2Command,
  GetObjectCommand,
  PutObjectCommand,
  CopyObjectCommand,
} from '@aws-sdk/client-s3'
import sharp from 'sharp'

const BUCKET = process.env.MEDIA_BUCKET
const REGION = process.env.AWS_REGION ?? 'us-east-1'
const DRY_RUN = process.env.DRY_RUN === '1'
// Set TEST_KEY to process a single object and exit, e.g. "plants/01JX.../avatar/01JX..."
const TEST_KEY = process.env.TEST_KEY ?? null

if (!BUCKET) {
  console.error('MEDIA_BUCKET env var required')
  process.exit(1)
}

const s3 = new S3Client({ region: REGION })

async function streamToBuffer(stream) {
  const chunks = []
  for await (const chunk of stream) chunks.push(chunk)
  return Buffer.concat(chunks)
}

async function listAllObjects() {
  const objects = []
  let token
  do {
    const res = await s3.send(new ListObjectsV2Command({
      Bucket: BUCKET,
      ContinuationToken: token,
    }))
    objects.push(...(res.Contents ?? []))
    token = res.NextContinuationToken
  } while (token)
  return objects
}

async function processKey(key) {
  // Download original
  const get = await s3.send(new GetObjectCommand({ Bucket: BUCKET, Key: key }))
  const original = await streamToBuffer(get.Body)
  const originalMB = (original.length / 1024 / 1024).toFixed(1)

  // Compress
  let compressed
  try {
    compressed = await sharp(original)
      .resize({ width: 1920, height: 1920, fit: 'inside', withoutEnlargement: true })
      .jpeg({ quality: 85 })
      .toBuffer()
  } catch (e) {
    console.log(`  SKIP  ${key} — not a supported image (${e.message})`)
    return
  }

  const compressedKB = (compressed.length / 1024).toFixed(0)
  const saving = Math.round((1 - compressed.length / original.length) * 100)
  console.log(`  ${originalMB} MB → ${compressedKB} KB  (${saving}% smaller)  ${key}`)

  if (DRY_RUN) return

  // Back up original
  await s3.send(new CopyObjectCommand({
    Bucket: BUCKET,
    CopySource: encodeURIComponent(`${BUCKET}/${key}`),
    Key: `originals/${key}`,
  }))

  // Overwrite with compressed
  await s3.send(new PutObjectCommand({
    Bucket: BUCKET,
    Key: key,
    Body: compressed,
    ContentType: 'image/jpeg',
  }))
}

async function main() {
  console.log(`Bucket: ${BUCKET}  Region: ${REGION}${DRY_RUN ? '  [DRY RUN]' : ''}`)

  if (TEST_KEY) {
    console.log(`\nTest mode — processing single key: ${TEST_KEY}\n`)
    await processKey(TEST_KEY)
    console.log('\nDone. Check the image then run without TEST_KEY to process everything.')
    return
  }

  const all = await listAllObjects()
  // Skip anything already in the originals/ backup prefix
  const images = all.filter(o => !o.Key.startsWith('originals/'))
  console.log(`\nFound ${images.length} objects\n`)

  let processed = 0
  let skipped = 0
  for (const obj of images) {
    try {
      await processKey(obj.Key)
      processed++
    } catch (e) {
      console.error(`  ERROR ${obj.Key}: ${e.message}`)
      skipped++
    }
  }

  console.log(`\nDone. ${processed} processed, ${skipped} errors.`)
  if (!DRY_RUN) {
    console.log(`Originals backed up under s3://${BUCKET}/originals/`)
  }
}

main().catch(e => { console.error(e); process.exit(1) })
