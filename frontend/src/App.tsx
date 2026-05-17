import { Routes, Route, Navigate } from 'react-router-dom'
import { DateStrip } from '@/components/DateStrip'
import { BottomNav } from '@/components/BottomNav'
import { GardenPage } from '@/pages/Garden'
import { PlantsPage } from '@/pages/Plants'
import { EnvironmentsPage } from '@/pages/Environments'
import { PlantDetailPage } from '@/pages/PlantDetail'

export default function App() {
  return (
    <div className="flex flex-col h-[100dvh] bg-base">
      <DateStrip />

      <main className="flex-1 overflow-y-auto">
        <Routes>
          <Route path="/"               element={<Navigate to="/garden" replace />} />
          <Route path="/garden"         element={<GardenPage />} />
          <Route path="/plants"         element={<PlantsPage />} />
          <Route path="/plants/:id"     element={<PlantDetailPage />} />
          <Route path="/environments"   element={<EnvironmentsPage />} />
        </Routes>
      </main>

      <BottomNav />
    </div>
  )
}
