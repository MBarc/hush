import { useEffect, useState, createContext, useContext } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { api } from './api'
import { ToastProvider } from './components/ui'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Shell from './pages/Shell'
import Browse from './pages/Browse'
import Devices from './pages/Devices'
import Users from './pages/Users'
import Audit from './pages/Audit'

const MeCtx = createContext(null)
export const useMe = () => useContext(MeCtx)

export default function App() {
  const [me, setMe] = useState(undefined) // undefined = loading, null = logged out
  const location = useLocation()

  const refresh = () =>
    api
      .me()
      .then(setMe)
      .catch(() => setMe(null))

  useEffect(() => {
    refresh()
  }, [])

  if (me === undefined) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted">
        <span className="mono text-sm">opening the vault...</span>
      </div>
    )
  }

  return (
    <ToastProvider>
      <MeCtx.Provider value={{ me, setMe, refresh }}>
        {me === null ? (
          <Routes>
            <Route path="*" element={<Login onLogin={setMe} />} />
          </Routes>
        ) : (
          <Shell>
            <ErrorBoundary key={location.pathname}>
              <Routes>
                <Route path="/" element={<Browse />} />
                <Route path="/browse/*" element={<Browse />} />
                <Route path="/devices" element={<Devices />} />
                <Route path="/users" element={me.admin ? <Users /> : <Navigate to="/" />} />
                <Route path="/audit" element={me.admin ? <Audit /> : <Navigate to="/" />} />
                <Route path="*" element={<Navigate to="/" />} />
              </Routes>
            </ErrorBoundary>
          </Shell>
        )}
      </MeCtx.Provider>
    </ToastProvider>
  )
}
