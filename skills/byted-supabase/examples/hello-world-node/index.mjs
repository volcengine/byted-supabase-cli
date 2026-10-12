import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'
import { Sandbox } from 'e2b'

function required(name) {
  const value = process.env[name]
  if (!value) {
    throw new Error(`missing environment variable: ${name}`)
  }
  return value
}

function optional(name) {
  return process.env[name]?.trim() || undefined
}

async function authRequest(url, path, body) {
  const response = await fetch(`${url}/auth/v1/${path}`, {
    method: 'POST',
    headers: {
      apikey: required('SUPABASE_ANON_KEY'),
      'content-type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  const responseBody = await response.json().catch(() => ({}))
  return { response, body: responseBody }
}

async function resolveAccessToken(url) {
  const accessToken = optional('SUPABASE_ACCESS_TOKEN')
  if (accessToken) return accessToken

  const email = required('SUPABASE_EMAIL')
  const password = required('SUPABASE_PASSWORD')

  const signup = await authRequest(url, 'signup', { email, password })
  if (!signup.response.ok && signup.response.status !== 422) {
    throw new Error(
      `Supabase Auth signup failed (${signup.response.status}): ${
        signup.body.msg ?? signup.body.error_description ?? 'unknown error'
      }`,
    )
  }

  const login = await authRequest(url, 'token?grant_type=password', {
    email,
    password,
  })
  if (!login.response.ok || !login.body.access_token) {
    throw new Error(
      `Supabase Auth login failed (${login.response.status}): ${
        login.body.msg ?? login.body.error_description ?? 'access_token was not returned'
      }. If email confirmation is enabled, confirm the test account before running again.`,
    )
  }
  return login.body.access_token
}

const supabaseUrl = required('SUPABASE_URL')
initBytedSupabaseSandbox({
  url: supabaseUrl,
  apiKey: await resolveAccessToken(supabaseUrl),
})

let sandbox
try {
  sandbox = await Sandbox.create('base', {
    timeoutMs: 5 * 60 * 1000,
  })

  const result = await sandbox.commands.run(
    "python3 -c 'print(\"hello world\")'",
  )
  const stdout = result.stdout ?? ''
  const stderr = result.stderr ?? ''

  if (stdout) process.stdout.write(stdout)
  if (stderr) process.stderr.write(stderr)
  if (result.exitCode !== undefined && result.exitCode !== 0) {
    throw new Error(`sandbox command failed with exit code ${result.exitCode}`)
  }
  if (!stdout.includes('hello world')) {
    throw new Error('sandbox verification failed: expected output was not found')
  }
  console.log('sandbox verification passed')
} finally {
  if (sandbox) await sandbox.kill()
}
