/**
 * Point the official E2B SDK at a Volcengine Supabase Sandbox instance.
 *
 *     import { initBytedSupabaseSandbox } from '@byted-supabase/sandbox'
 *
 *     initBytedSupabaseSandbox({ url: 'https://<your-instance>', apiKey: '<supabase jwt>' })
 *
 *     import { Sandbox } from 'e2b'
 *     const sbx = await Sandbox.create()
 *
 * Everything after that call is stock E2B usage -- same imports, same API, same documentation.
 */

export * from './e2b_gateway'
