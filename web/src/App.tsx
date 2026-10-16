import { Route, Routes } from 'react-router'
import { Layout } from './components/Layout'
import Nodes from './pages/Nodes'
import Overview from './pages/Overview'
import PotDetail from './pages/PotDetail'
import PotNew from './pages/PotNew'
import PotSettings from './pages/PotSettings'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/pots/new" element={<PotNew />} />
        <Route path="/pots/:id" element={<PotDetail />} />
        <Route path="/pots/:id/settings" element={<PotSettings />} />
        <Route path="/nodes" element={<Nodes />} />
      </Route>
    </Routes>
  )
}
