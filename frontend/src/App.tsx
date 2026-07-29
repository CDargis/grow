import { Routes, Route, Navigate } from 'react-router-dom'
import { BottomNav } from '@/components/BottomNav'
import { PlantsPage } from '@/pages/Plants'
import { EnvironmentsPage } from '@/pages/Environments'
import { PlantDetailPage } from '@/pages/PlantDetail'
import { OverviewPage } from '@/pages/Overview'
import { InventoryPage } from '@/pages/Inventory'
import { AuthProvider } from '@/auth/AuthProvider'

export default function App() {
  return (
    <AuthProvider>
      <div className="flex flex-col h-[100dvh] bg-base">
        <main className="flex-1 min-h-0 overflow-y-auto">
          <Routes>
            <Route path="/"               element={<Navigate to="/plants" replace />} />
            <Route path="/plants"         element={<PlantsPage />} />
            <Route path="/plants/:id"     element={<PlantDetailPage />} />
            <Route path="/environments"   element={<EnvironmentsPage />} />
            <Route path="/overview"       element={<OverviewPage />} />
            <Route path="/inventory"      element={<InventoryPage />} />
          </Routes>
        </main>

        <BottomNav />
      </div>
    </AuthProvider>
  )
}
