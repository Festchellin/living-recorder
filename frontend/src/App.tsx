import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import Dashboard from '@/pages/Dashboard'
import Streams from '@/pages/Streams'
import StreamDetail from '@/pages/StreamDetail'
import Tasks from '@/pages/Tasks'
import Settings from '@/pages/Settings'

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/streams" element={<Streams />} />
          <Route path="/streams/:id" element={<StreamDetail />} />
          <Route path="/tasks" element={<Tasks />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
