import { useNavigate } from 'react-router-dom'
import { Home } from 'lucide-react'

export default function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="h-full flex items-center justify-center">
      <div className="text-center">
        <p className="text-6xl font-bold text-gray-200 dark:text-gray-700">404</p>
        <p className="text-lg text-gray-500 dark:text-gray-400 mt-2">页面不存在</p>
        <button
          onClick={() => navigate('/dashboard')}
          className="mt-6 inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700"
        >
          <Home className="w-4 h-4" /> 返回首页
        </button>
      </div>
    </div>
  )
}
