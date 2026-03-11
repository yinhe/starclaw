import { Link } from 'react-router-dom';
import { ArrowLeft, MapPin, Mail, Phone } from 'lucide-react';

export default function PrivacyPage() {
  return (
    <div className="min-h-screen bg-gray-950 text-gray-300">
      <div className="max-w-3xl mx-auto px-6 py-16">
        <Link to="/" className="inline-flex items-center gap-2 text-gray-500 hover:text-gray-300 text-sm mb-8 transition-colors">
          <ArrowLeft className="w-4 h-4" /> 返回首页
        </Link>

        <h1 className="text-3xl font-bold text-white mb-2">隐私政策</h1>
        <p className="text-gray-500 text-sm mb-10">最后更新：2026 年 3 月 11 日</p>

        <div className="space-y-8 text-sm leading-relaxed">
          <section>
            <h2 className="text-lg font-semibold text-white mb-3">1. 引言</h2>
            <p>浙江银河天启科技有限公司（以下简称"我们"）重视用户隐私。本隐私政策说明我们在您使用 Star-AI 平台（以下简称"本平台"）时如何收集、使用、存储和保护您的个人信息。</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">2. 信息收集</h2>
            <p className="mb-3">我们收集以下类型的信息：</p>
            <h3 className="text-white font-medium mb-2">2.1 注册信息</h3>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400 mb-4">
              <li>邮箱地址（必填，用于账户识别和登录）</li>
              <li>用户昵称（可选）</li>
              <li>密码（加密存储，使用 bcrypt 哈希）</li>
            </ul>
            <h3 className="text-white font-medium mb-2">2.2 使用数据</h3>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400 mb-4">
              <li>API 调用记录（模型、Token 用量、时间戳、耗时）</li>
              <li>充值和消费记录</li>
              <li>IP 地址和设备信息（用于安全防护）</li>
            </ul>
            <h3 className="text-white font-medium mb-2">2.3 我们不收集的信息</h3>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400">
              <li>我们 <span className="text-white font-medium">不会存储</span> 您的 API 请求和响应内容（prompt 和 completion）</li>
              <li>我们不会收集您的身份证、手机号等敏感个人信息</li>
              <li>我们不会通过 Cookie 追踪您的浏览行为</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">3. 信息使用</h2>
            <p className="mb-2">我们将收集的信息用于：</p>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400">
              <li>提供和维护平台服务（身份认证、API 路由、计费扣费）</li>
              <li>生成用量统计和账单（不涉及内容本身）</li>
              <li>安全防护（异常检测、防欺诈、DDoS 防护）</li>
              <li>服务改进和性能优化</li>
              <li>发送重要通知（账户安全、服务变更、余额不足提醒）</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">4. 信息存储与安全</h2>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400">
              <li>所有数据存储在中国境内的服务器上</li>
              <li>密码使用 bcrypt 算法加密存储</li>
              <li>API Key 使用 SHA-256 哈希存储，明文仅在创建时显示一次</li>
              <li>所有 API 通信使用 HTTPS/TLS 加密</li>
              <li>数据库定期备份，防止数据丢失</li>
              <li>我们采取合理的技术和管理措施保护您的信息安全</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">5. 信息共享</h2>
            <p className="mb-2">我们不会向第三方出售您的个人信息。以下情况除外：</p>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400">
              <li><span className="text-white">上游 API 调用：</span>您的请求内容会转发给相应的 AI 模型提供商（OpenAI、Anthropic、阿里云等），受各提供商隐私政策约束</li>
              <li><span className="text-white">支付处理：</span>充值时订单信息会发送给支付宝/微信支付用于交易处理</li>
              <li><span className="text-white">法律要求：</span>应法律法规、司法程序或政府要求提供</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">6. 用户权利</h2>
            <p className="mb-2">您有权：</p>
            <ul className="list-disc list-inside space-y-1.5 text-gray-400">
              <li>查看和导出您的账户信息和使用记录</li>
              <li>修改您的账户信息（邮箱、昵称、密码）</li>
              <li>删除您的 API Key</li>
              <li>申请注销账户（联系 support@star-ai.net）</li>
              <li>账户注销后，我们将在 30 日内删除您的个人数据（法律要求保留的除外）</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">7. 未成年人保护</h2>
            <p>本平台不面向 18 岁以下的未成年人。如果我们发现未成年人注册使用，将立即注销其账户。</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">8. 政策更新</h2>
            <p>我们可能不时更新本隐私政策。更新后的版本将在平台公布，重大变更将通过邮件通知您。继续使用本平台即表示您接受更新后的政策。</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">9. 联系我们</h2>
            <p className="text-gray-400 mb-5">有任何问题或建议，欢迎通过以下方式与我们联系。我们期待为您提供帮助！</p>
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-4">
              <div className="flex items-center gap-3 text-gray-300">
                <MapPin className="w-5 h-5 text-amber-400 shrink-0" />
                <span>地址：浙江杭州萧山万象会写字楼B座1016A</span>
              </div>
              <div className="flex items-center gap-3 text-gray-300">
                <Mail className="w-5 h-5 text-amber-400 shrink-0" />
                <span>邮箱：<a href="mailto:service@star-ai.net" className="text-amber-400 hover:text-amber-300">service@star-ai.net</a></span>
              </div>
              <div className="flex items-center gap-3 text-gray-300">
                <Phone className="w-5 h-5 text-amber-400 shrink-0" />
                <span>电话：+86 133 9671 7829</span>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
