import { UserManager, WebStorageStateStore, type UserManagerSettings } from 'oidc-client-ts'

interface AuthConfig {
  authority: string
  clientId: string
}

let managerPromise: Promise<UserManager> | null = null

async function fetchAuthConfig(): Promise<AuthConfig> {
  const res = await fetch('/api/auth-config')
  if (!res.ok) throw new Error('failed to load auth config')
  return res.json()
}

// The Cognito User Pool / Client id don't exist until after the CDK stack
// that creates them is deployed, so they can't be baked in at frontend
// build time -- fetched at runtime from the one unauthenticated endpoint
// instead. Cached so this only happens once per page load.
export function getUserManager(): Promise<UserManager> {
  if (!managerPromise) {
    managerPromise = fetchAuthConfig().then(config => {
      const settings: UserManagerSettings = {
        authority: config.authority,
        client_id: config.clientId,
        redirect_uri: `${window.location.origin}/callback`,
        post_logout_redirect_uri: `${window.location.origin}/`,
        response_type: 'code',
        scope: 'openid email profile',
        automaticSilentRenew: true,
        userStore: new WebStorageStateStore({ store: window.localStorage }),
      }
      return new UserManager(settings)
    })
  }
  return managerPromise
}
