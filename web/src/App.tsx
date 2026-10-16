import { Route, Routes } from 'react-router'
import { Layout } from './components/Layout'
import Overview from './pages/Overview'
import PotDetail from './pages/PotDetail'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/pots/new" element={<p>New pot</p>} />
        <Route path="/pots/:id" element={<PotDetail />} />
        <Route path="/pots/:id/settings" element={<p>Pot settings</p>} />
        <Route path="/nodes" element={<p>Nodes</p>} />
      </Route>
    </Routes>
  )
}
