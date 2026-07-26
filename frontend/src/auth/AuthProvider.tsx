import { useEffect, useState } from 'react'
import { getUserManager } from './userManager'

interface Props {
  children: React.ReactNode
}

// Personal single-user app -- there's no "please log in" landing page.
// If there's no valid session, redirect straight to the Cognito Hosted UI.
export function AuthProvider({ children }: Props) {
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let cancelled = false

    getUserManager().then(async manager => {
      if (window.location.pathname === '/callback') {
        await manager.signinRedirectCallback()
        window.history.replaceState({}, '', '/')
        if (!cancelled) setReady(true)
        return
      }

      const user = await manager.getUser()
      if (user && !user.expired) {
        if (!cancelled) setReady(true)
        return
      }

      // The access token is expired or missing, but a refresh token from an
      // earlier session may still be valid (automaticSilentRenew only fires
      // while the tab stays open continuously -- closing the browser for
      // over an hour otherwise forced a full interactive re-login every
      // time). Try the refresh token first; only redirect to a full login
      // if that fails (refresh token also expired/revoked, or no prior
      // session at all).
      if (user) {
        try {
          await manager.signinSilent()
          if (!cancelled) setReady(true)
          return
        } catch (err) {
          console.warn('silent renew failed, falling back to full login', err)
        }
      }

      await manager.signinRedirect()
    }).catch(err => {
      console.error('auth init failed', err)
    })

    return () => { cancelled = true }
  }, [])

  if (!ready) {
    return (
      <div className="flex items-center justify-center h-[100dvh] bg-base text-muted text-sm">
        Loading…
      </div>
    )
  }

  return <>{children}</>
}
