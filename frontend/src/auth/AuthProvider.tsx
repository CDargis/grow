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
