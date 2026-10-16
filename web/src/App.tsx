import { Route, Routes } from 'react-router'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<main>Pots</main>} />
      <Route path="/pots/:id" element={<main>Pot detail</main>} />
      <Route path="/nodes" element={<main>Nodes</main>} />
    </Routes>
  )
}
