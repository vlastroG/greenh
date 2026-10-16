import { NavLink, Outlet } from 'react-router'

const link = ({ isActive }: { isActive: boolean }) =>
  `rounded-md px-3 py-1.5 text-sm font-medium ${isActive ? 'bg-green-100 text-green-800' : 'text-gray-600 hover:bg-gray-100'}`

export function Layout() {
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900">
      <header className="border-b border-gray-200 bg-white">
        <nav className="mx-auto flex max-w-5xl items-center gap-2 px-4 py-3">
          <span className="mr-4 font-semibold text-green-700">Plant waterer</span>
          <NavLink to="/" end className={link}>
            Pots
          </NavLink>
          <NavLink to="/nodes" className={link}>
            Nodes
          </NavLink>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
