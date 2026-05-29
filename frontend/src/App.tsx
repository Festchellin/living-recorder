import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import Dashboard from '@/pages/Dashboard'
import Streams from '@/pages/Streams'
import StreamDetail from '@/pages/StreamDetail'
import SettingsPage from '@/pages/Settings'
import Groups from '@/pages/Groups'
import Preview from '@/pages/Preview'
import Logs from '@/pages/Logs'

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/streams" element={<Streams />} />
          <Route path="/streams/:id" element={<StreamDetail />} />
          <Route path="/preview" element={<Preview />} />
          <Route path="/groups" element={<Groups />} />
          <Route path="/logs" element={<Logs />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
