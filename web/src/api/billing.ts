import { http } from './http';

export interface UserBalance {
  user_id: string;
  balance: number;
  gift_balance: number;
  total_consumed: number;
}

export interface BillingLog {
  id?: number;
  request_id: string;
  user_id: string;
  service_type: string;
  provider: string;
  model_name: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  doc_count: number;
  total_cost: number;
  status: number; // 1-已扣费, 2-已退款
  created_at: number | string;
}

export interface ModelPricing {
  input_unit_price: number;
  output_unit_price: number;
  unit_size: number;
}

export interface ListLogsResult {
  total: number;
  logs: BillingLog[];
  page: number;
  limit: number;
}

export const billingApi = {
  // 获取当前用户余额
  async getBalance(): Promise<UserBalance> {
    const raw: any = await http.get('/base/v1/billing/getBalance');
    return {
      user_id: raw?.user_id ?? raw?.userId ?? '',
      balance: Number(raw?.balance ?? 0),
      gift_balance: Number(raw?.gift_balance ?? raw?.giftBalance ?? 0),
      total_consumed: Number(raw?.total_consumed ?? raw?.totalConsumed ?? 0),
    };
  },

  // 用户充值/体验额度赠送
  async recharge(amount: number, userId?: string): Promise<UserBalance> {
    const raw: any = await http.post('/base/v1/billing/recharge', { amount, user_id: userId });
    return {
      user_id: raw?.user_id ?? raw?.userId ?? '',
      balance: Number(raw?.balance ?? 0),
      gift_balance: Number(raw?.gift_balance ?? raw?.giftBalance ?? 0),
      total_consumed: Number(raw?.total_consumed ?? raw?.totalConsumed ?? 0),
    };
  },

  // 查询消耗明细流水
  async listLogs(page = 1, limit = 20): Promise<ListLogsResult> {
    const raw: any = await http.get('/base/v1/billing/listLogs', {
      params: { page, limit },
    });

    const rawLogs = raw?.logs ?? [];
    const logs: BillingLog[] = rawLogs.map((item: any) => ({
      id: item?.id ?? item?.request_id ?? item?.requestId,
      request_id: item?.request_id ?? item?.requestId ?? '',
      user_id: item?.user_id ?? item?.userId ?? '',
      service_type: item?.service_type ?? item?.serviceType ?? 'openai',
      provider: item?.provider ?? '',
      model_name: item?.model_name ?? item?.modelName ?? '',
      prompt_tokens: Number(item?.prompt_tokens ?? item?.promptTokens ?? 0),
      completion_tokens: Number(item?.completion_tokens ?? item?.completionTokens ?? 0),
      total_tokens: Number(item?.total_tokens ?? item?.totalTokens ?? 0),
      doc_count: Number(item?.doc_count ?? item?.docCount ?? 0),
      total_cost: Number(item?.total_cost ?? item?.totalCost ?? 0),
      status: Number(item?.status ?? 1),
      created_at: item?.created_at ?? item?.createdAt ?? Date.now(),
    }));

    return {
      total: Number(raw?.total ?? 0),
      page: Number(raw?.page ?? page),
      limit: Number(raw?.limit ?? limit),
      logs,
    };
  },

  // 获取模型单价规则表
  async getPricing(): Promise<Record<string, ModelPricing>> {
    const raw: any = await http.get('/base/v1/billing/getPricing');
    const rawPrices = raw?.prices ?? raw ?? {};

    const resultMap: Record<string, ModelPricing> = {};
    for (const [key, val] of Object.entries(rawPrices)) {
      if (val && typeof val === 'object') {
        const item = val as any;
        resultMap[key] = {
          input_unit_price: Number(item.input_unit_price ?? item.inputUnitPrice ?? 0),
          output_unit_price: Number(item.output_unit_price ?? item.outputUnitPrice ?? 0),
          unit_size: Number(item.unit_size ?? item.unitSize ?? 1000),
        };
      }
    }
    return resultMap;
  },
};
