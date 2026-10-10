/**
 * Complete Node.js SDK example:
 *   init -> create -> write -> execute -> verify -> kill
 *
 * Required environment variables:
 *   SUPABASE_URL
 *   SUPABASE_SERVICE_ROLE_KEY
 *   SANDBOX_OWNER_USER_ID
 *
 * Run this file only in a backend or trusted worker. Never expose the
 * service-role key to a browser or commit real credentials.
 */
import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'
import { Sandbox } from 'e2b'

function required(name) {
  const value = process.env[name]
  if (!value) {
    throw new Error(`missing environment variable: ${name}`)
  }
  return value
}

const url = required('SUPABASE_URL')
const serviceRoleKey = required('SUPABASE_SERVICE_ROLE_KEY')
const ownerUserId = required('SANDBOX_OWNER_USER_ID')

initBytedSupabaseSandbox({
  url,
  apiKey: serviceRoleKey,
})

let sandbox
let passed = false

try {
  console.log('creating sandbox')
  sandbox = await Sandbox.create('base', {
    timeoutMs: 10 * 60 * 1000,
    metadata: { ownerUserId },
  })

  await sandbox.files.write('/tmp/hello.py', 'print("hello from Supabase Sandbox")\n')
  const result = await sandbox.commands.run('python3 /tmp/hello.py')

  const stdout = result.stdout ?? ''
  const stderr = result.stderr ?? ''
  const exitCode = result.exitCode
  if (stdout) process.stdout.write(stdout)
  if (stderr) process.stderr.write(stderr)

  if (exitCode !== undefined && exitCode !== 0) {
    throw new Error(`sandbox command failed with exit code ${exitCode}`)
  }
  if (!stdout.includes('hello from Supabase Sandbox')) {
    throw new Error('sandbox verification failed: expected output was not found')
  }
  passed = true
  console.log('sandbox verification passed')
} finally {
  if (sandbox) {
    console.log('releasing sandbox')
    await sandbox.kill()
  }
}

if (!passed) {
  process.exitCode = 1
}
