import { Link } from 'react-router-dom';
import { ArrowLeft, MapPin, Mail, Phone } from 'lucide-react';

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-gray-950 text-gray-300">
      <div className="max-w-3xl mx-auto px-6 py-16">
        <Link to="/" className="inline-flex items-center gap-2 text-gray-500 hover:text-gray-300 text-sm mb-8 transition-colors">
          <ArrowLeft className="w-4 h-4" /> 返回首页
        </Link>

        <h1 className="text-3xl font-bold text-white mb-2">服务条款</h1>
        <p className="text-gray-500 text-sm mb-10">最后更新：2026 年 3 月 11 日</p>

        <div className="space-y-8 text-sm leading-relaxed">
          <section>
            <h2 className="text-lg font-semibold text-white mb-3">1. 服务概述</h2>
            <p>Star-AI（以下简称"本平台"）是由浙江银河天启科技有限公司运营的 AI 算力服务平台，通过统一的 API 接口为用户提供多种 AI 模型的调用服务。</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">2. 账户注册</h2>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>用户需使用有效邮箱注册账户，并设置安全密码。</li>
              <li>每位用户仅可注册一个账户，不得转让或出借账户。</li>
              <li>用户应妥善保管 API Key，因泄露造成的损失由用户自行承担。</li>
              <li>注册即视为同意本服务条款及隐私政策。</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">3. 服务内容</h2>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>本平台提供 OpenAI 兼容的 API 接口，支持多种 AI 模型调用。</li>
              <li>模型列表、定价和可用性可能随时调整，调整前会在平台公告。</li>
              <li>本平台作为算力中间层，不保证第三方模型的输出质量和准确性。</li>
              <li>免费额度仅供试用，不可转让或提现。</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">4. 计费与支付</h2>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>本平台采用预付费模式，用户需先充值后使用。</li>
              <li>费用按实际 API 调用的 Token 用量或调用次数计算。</li>
              <li>充值金额一经到账不予退款，充值赠送部分不可提现。</li>
              <li>本平台保留调整定价的权利，价格变动前 7 日公告。</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">5. 使用规范</h2>
            <p className="mb-2">用户不得将本平台用于以下用途：</p>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>违反中华人民共和国法律法规的任何活动。</li>
              <li>生成、传播违法、暴力、色情、欺诈等有害内容。</li>
              <li>恶意攻击、干扰平台正常运行（DDoS、暴力破解等）。</li>
              <li>未经授权爬取或转售平台服务。</li>
              <li>利用 API 进行自动化垃圾信息、骚扰或其他滥用行为。</li>
            </ul>
            <p className="mt-3">违反以上规定的，本平台有权立即停用账户并扣除余额，必要时追究法律责任。</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">6. 服务可用性</h2>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>本平台尽力保障 99.9% 的服务可用性，但不作绝对承诺。</li>
              <li>因不可抗力（网络故障、上游服务商中断等）导致的服务中断，不承担赔偿责任。</li>
              <li>计划维护将提前 24 小时通知用户。</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">7. 知识产权</h2>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>本平台的品牌、界面设计、技术实现等知识产权归浙江银河天启科技有限公司所有。</li>
              <li>用户通过 API 生成的内容，其权利归用户所有，但需遵守各模型提供商的使用协议。</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">8. 免责声明</h2>
            <ul className="list-disc list-inside space-y-2 text-gray-400">
              <li>AI 模型生成的内容仅供参考，不构成专业建议。</li>
              <li>本平台不对模型输出的准确性、完整性或适用性做任何担保。</li>
              <li>因用户使用模型输出所产生的任何后果，由用户自行承担。</li>
            </ul>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">9. 条款修改</h2>
            <p>本平台保留随时修改本条款的权利。修改后的条款将在平台上公布，继续使用本服务即视为接受修改后的条款。</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-white mb-3">10. 联系我们</h2>
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
