package supabasetypes

type PublicUserProfilesSelect struct {
  AvatarUrl   *string `json:"avatar_url"`
  CreatedAt   string  `json:"created_at"`
  DisplayName *string `json:"display_name"`
  Email       string  `json:"email"`
  Id          string  `json:"id"`
  Role        string  `json:"role"`
  UpdatedAt   string  `json:"updated_at"`
}

type PublicUserProfilesInsert struct {
  AvatarUrl   *string `json:"avatar_url"`
  CreatedAt   *string `json:"created_at"`
  DisplayName *string `json:"display_name"`
  Email       string  `json:"email"`
  Id          string  `json:"id"`
  Role        *string `json:"role"`
  UpdatedAt   *string `json:"updated_at"`
}

type PublicUserProfilesUpdate struct {
  AvatarUrl   *string `json:"avatar_url"`
  CreatedAt   *string `json:"created_at"`
  DisplayName *string `json:"display_name"`
  Email       *string `json:"email"`
  Id          *string `json:"id"`
  Role        *string `json:"role"`
  UpdatedAt   *string `json:"updated_at"`
}

type PublicExchangeCredentialsSelect struct {
  ApiKeyEncrypted     string  `json:"api_key_encrypted"`
  ApiSecretEncrypted  string  `json:"api_secret_encrypted"`
  CreatedAt           string  `json:"created_at"`
  Exchange            string  `json:"exchange"`
  Id                  string  `json:"id"`
  IsTestnet           bool    `json:"is_testnet"`
  IsVerified          bool    `json:"is_verified"`
  LastVerifiedAt      *string `json:"last_verified_at"`
  PassphraseEncrypted *string `json:"passphrase_encrypted"`
  UserId              string  `json:"user_id"`
}

type PublicExchangeCredentialsInsert struct {
  ApiKeyEncrypted     string  `json:"api_key_encrypted"`
  ApiSecretEncrypted  string  `json:"api_secret_encrypted"`
  CreatedAt           *string `json:"created_at"`
  Exchange            string  `json:"exchange"`
  Id                  *string `json:"id"`
  IsTestnet           *bool   `json:"is_testnet"`
  IsVerified          *bool   `json:"is_verified"`
  LastVerifiedAt      *string `json:"last_verified_at"`
  PassphraseEncrypted *string `json:"passphrase_encrypted"`
  UserId              string  `json:"user_id"`
}

type PublicExchangeCredentialsUpdate struct {
  ApiKeyEncrypted     *string `json:"api_key_encrypted"`
  ApiSecretEncrypted  *string `json:"api_secret_encrypted"`
  CreatedAt           *string `json:"created_at"`
  Exchange            *string `json:"exchange"`
  Id                  *string `json:"id"`
  IsTestnet           *bool   `json:"is_testnet"`
  IsVerified          *bool   `json:"is_verified"`
  LastVerifiedAt      *string `json:"last_verified_at"`
  PassphraseEncrypted *string `json:"passphrase_encrypted"`
  UserId              *string `json:"user_id"`
}

type PublicOrdersSelect struct {
  ActualOrderId      int64   `json:"actual_order_id"`
  ClientOrderId      string  `json:"client_order_id"`
  CreatedAt          string  `json:"created_at"`
  Exchange           string  `json:"exchange"`
  ExecutedQuantity   *string `json:"executed_quantity"`
  Id                 string  `json:"id"`
  IsFutures          bool    `json:"is_futures"`
  IsIsolated         bool    `json:"is_isolated"`
  IsMargin           bool    `json:"is_margin"`
  IsWorking          bool    `json:"is_working"`
  OrderId            string  `json:"order_id"`
  OrderType          string  `json:"order_type"`
  OrderUuid          string  `json:"order_uuid"`
  Price              string  `json:"price"`
  Quantity           string  `json:"quantity"`
  Side               string  `json:"side"`
  Status             string  `json:"status"`
  StopPrice          string  `json:"stop_price"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TimeInForce        string  `json:"time_in_force"`
  UpdatedAt          string  `json:"updated_at"`
  UserId             string  `json:"user_id"`
}

type PublicOrdersInsert struct {
  ActualOrderId      *int64  `json:"actual_order_id"`
  ClientOrderId      *string `json:"client_order_id"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  ExecutedQuantity   *string `json:"executed_quantity"`
  Id                 *string `json:"id"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMargin           *bool   `json:"is_margin"`
  IsWorking          *bool   `json:"is_working"`
  OrderId            string  `json:"order_id"`
  OrderType          string  `json:"order_type"`
  OrderUuid          *string `json:"order_uuid"`
  Price              string  `json:"price"`
  Quantity           string  `json:"quantity"`
  Side               string  `json:"side"`
  Status             string  `json:"status"`
  StopPrice          *string `json:"stop_price"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TimeInForce        *string `json:"time_in_force"`
  UpdatedAt          *string `json:"updated_at"`
  UserId             string  `json:"user_id"`
}

type PublicOrdersUpdate struct {
  ActualOrderId      *int64  `json:"actual_order_id"`
  ClientOrderId      *string `json:"client_order_id"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  ExecutedQuantity   *string `json:"executed_quantity"`
  Id                 *string `json:"id"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMargin           *bool   `json:"is_margin"`
  IsWorking          *bool   `json:"is_working"`
  OrderId            *string `json:"order_id"`
  OrderType          *string `json:"order_type"`
  OrderUuid          *string `json:"order_uuid"`
  Price              *string `json:"price"`
  Quantity           *string `json:"quantity"`
  Side               *string `json:"side"`
  Status             *string `json:"status"`
  StopPrice          *string `json:"stop_price"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TimeInForce        *string `json:"time_in_force"`
  UpdatedAt          *string `json:"updated_at"`
  UserId             *string `json:"user_id"`
}

type PublicTradesSelect struct {
  Exchange           string  `json:"exchange"`
  Fee                string  `json:"fee"`
  FeeCurrency        string  `json:"fee_currency"`
  Id                 string  `json:"id"`
  IsBuyer            bool    `json:"is_buyer"`
  IsFutures          bool    `json:"is_futures"`
  IsIsolated         bool    `json:"is_isolated"`
  IsMaker            bool    `json:"is_maker"`
  IsMargin           bool    `json:"is_margin"`
  OrderId            string  `json:"order_id"`
  OrderUuid          string  `json:"order_uuid"`
  Pnl                *string `json:"pnl"`
  Price              string  `json:"price"`
  Quantity           string  `json:"quantity"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               string  `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            string  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicTradesInsert struct {
  Exchange           *string `json:"exchange"`
  Fee                string  `json:"fee"`
  FeeCurrency        string  `json:"fee_currency"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  OrderId            string  `json:"order_id"`
  OrderUuid          *string `json:"order_uuid"`
  Pnl                *string `json:"pnl"`
  Price              string  `json:"price"`
  Quantity           string  `json:"quantity"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               string  `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            string  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicTradesUpdate struct {
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  OrderId            *string `json:"order_id"`
  OrderUuid          *string `json:"order_uuid"`
  Pnl                *string `json:"pnl"`
  Price              *string `json:"price"`
  Quantity           *string `json:"quantity"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *string `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             *string `json:"user_id"`
}

type PublicBacktestReportsSelect struct {
  Cagr         *string     `json:"cagr"`
  Config       interface{} `json:"config"`
  CreatedAt    string      `json:"created_at"`
  EndDate      string      `json:"end_date"`
  Id           string      `json:"id"`
  LossCount    int32       `json:"loss_count"`
  MaxDrawdown  string      `json:"max_drawdown"`
  ProfitFactor *string     `json:"profit_factor"`
  ReportJson   interface{} `json:"report_json"`
  SharpeRatio  *string     `json:"sharpe_ratio"`
  SortinoRatio *string     `json:"sortino_ratio"`
  StartDate    string      `json:"start_date"`
  Strategy     string      `json:"strategy"`
  TotalProfit  string      `json:"total_profit"`
  TotalTrades  int32       `json:"total_trades"`
  UserId       string      `json:"user_id"`
  WinCount     int32       `json:"win_count"`
  WinRate      string      `json:"win_rate"`
}

type PublicBacktestReportsInsert struct {
  Cagr         *string     `json:"cagr"`
  Config       interface{} `json:"config"`
  CreatedAt    *string     `json:"created_at"`
  EndDate      string      `json:"end_date"`
  Id           *string     `json:"id"`
  LossCount    int32       `json:"loss_count"`
  MaxDrawdown  string      `json:"max_drawdown"`
  ProfitFactor *string     `json:"profit_factor"`
  ReportJson   interface{} `json:"report_json"`
  SharpeRatio  *string     `json:"sharpe_ratio"`
  SortinoRatio *string     `json:"sortino_ratio"`
  StartDate    string      `json:"start_date"`
  Strategy     string      `json:"strategy"`
  TotalProfit  string      `json:"total_profit"`
  TotalTrades  int32       `json:"total_trades"`
  UserId       string      `json:"user_id"`
  WinCount     int32       `json:"win_count"`
  WinRate      string      `json:"win_rate"`
}

type PublicBacktestReportsUpdate struct {
  Cagr         *string     `json:"cagr"`
  Config       interface{} `json:"config"`
  CreatedAt    *string     `json:"created_at"`
  EndDate      *string     `json:"end_date"`
  Id           *string     `json:"id"`
  LossCount    *int32      `json:"loss_count"`
  MaxDrawdown  *string     `json:"max_drawdown"`
  ProfitFactor *string     `json:"profit_factor"`
  ReportJson   interface{} `json:"report_json"`
  SharpeRatio  *string     `json:"sharpe_ratio"`
  SortinoRatio *string     `json:"sortino_ratio"`
  StartDate    *string     `json:"start_date"`
  Strategy     *string     `json:"strategy"`
  TotalProfit  *string     `json:"total_profit"`
  TotalTrades  *int32      `json:"total_trades"`
  UserId       *string     `json:"user_id"`
  WinCount     *int32      `json:"win_count"`
  WinRate      *string     `json:"win_rate"`
}

type PublicPositionsSelect struct {
  AverageCost        string  `json:"average_cost"`
  Base               string  `json:"base"`
  BaseCurrency       string  `json:"base_currency"`
  CreatedAt          string  `json:"created_at"`
  Exchange           string  `json:"exchange"`
  Id                 string  `json:"id"`
  NetProfit          *string `json:"net_profit"`
  Profit             *string `json:"profit"`
  Quote              string  `json:"quote"`
  QuoteCurrency      string  `json:"quote_currency"`
  Side               string  `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPositionsInsert struct {
  AverageCost        *string `json:"average_cost"`
  Base               *string `json:"base"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Id                 *string `json:"id"`
  NetProfit          *string `json:"net_profit"`
  Profit             *string `json:"profit"`
  Quote              *string `json:"quote"`
  QuoteCurrency      *string `json:"quote_currency"`
  Side               *string `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPositionsUpdate struct {
  AverageCost        *string `json:"average_cost"`
  Base               *string `json:"base"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Id                 *string `json:"id"`
  NetProfit          *string `json:"net_profit"`
  Profit             *string `json:"profit"`
  Quote              *string `json:"quote"`
  QuoteCurrency      *string `json:"quote_currency"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *int64  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             *string `json:"user_id"`
}

type PublicProfitsSelect struct {
  AverageCost        string  `json:"average_cost"`
  BaseCurrency       string  `json:"base_currency"`
  CreatedAt          string  `json:"created_at"`
  Exchange           string  `json:"exchange"`
  Fee                string  `json:"fee"`
  FeeCurrency        string  `json:"fee_currency"`
  FeeInUsd           *string `json:"fee_in_usd"`
  Id                 string  `json:"id"`
  IsBuyer            bool    `json:"is_buyer"`
  IsFutures          bool    `json:"is_futures"`
  IsIsolated         bool    `json:"is_isolated"`
  IsMaker            bool    `json:"is_maker"`
  IsMargin           bool    `json:"is_margin"`
  NetProfit          string  `json:"net_profit"`
  NetProfitMargin    string  `json:"net_profit_margin"`
  Price              string  `json:"price"`
  Profit             string  `json:"profit"`
  ProfitMargin       string  `json:"profit_margin"`
  Quantity           string  `json:"quantity"`
  QuoteCurrency      string  `json:"quote_currency"`
  QuoteQuantity      string  `json:"quote_quantity"`
  Side               string  `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicProfitsInsert struct {
  AverageCost        *string `json:"average_cost"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  FeeInUsd           *string `json:"fee_in_usd"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  NetProfit          *string `json:"net_profit"`
  NetProfitMargin    *string `json:"net_profit_margin"`
  Price              *string `json:"price"`
  Profit             *string `json:"profit"`
  ProfitMargin       *string `json:"profit_margin"`
  Quantity           *string `json:"quantity"`
  QuoteCurrency      *string `json:"quote_currency"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicProfitsUpdate struct {
  AverageCost        *string `json:"average_cost"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  FeeInUsd           *string `json:"fee_in_usd"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  NetProfit          *string `json:"net_profit"`
  NetProfitMargin    *string `json:"net_profit_margin"`
  Price              *string `json:"price"`
  Profit             *string `json:"profit"`
  ProfitMargin       *string `json:"profit_margin"`
  Quantity           *string `json:"quantity"`
  QuoteCurrency      *string `json:"quote_currency"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *int64  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             *string `json:"user_id"`
}

type PublicStrategyRegistrySelect struct {
  Category        string      `json:"category"`
  CreatedAt       *string     `json:"created_at"`
  CrossExchange   *bool       `json:"cross_exchange"`
  Defaults        interface{} `json:"defaults"`
  Description     *string     `json:"description"`
  DisplayName     string      `json:"display_name"`
  Enabled         *bool       `json:"enabled"`
  Exchanges       interface{} `json:"exchanges"`
  Fields          interface{} `json:"fields"`
  Id              string      `json:"id"`
  LiveOnly        *bool       `json:"live_only"`
  RequiresFutures *bool       `json:"requires_futures"`
  SessionRoles    interface{} `json:"session_roles"`
  SortOrder       *int32      `json:"sort_order"`
  UpdatedAt       *string     `json:"updated_at"`
}

type PublicStrategyRegistryInsert struct {
  Category        *string     `json:"category"`
  CreatedAt       *string     `json:"created_at"`
  CrossExchange   *bool       `json:"cross_exchange"`
  Defaults        interface{} `json:"defaults"`
  Description     *string     `json:"description"`
  DisplayName     string      `json:"display_name"`
  Enabled         *bool       `json:"enabled"`
  Exchanges       interface{} `json:"exchanges"`
  Fields          interface{} `json:"fields"`
  Id              string      `json:"id"`
  LiveOnly        *bool       `json:"live_only"`
  RequiresFutures *bool       `json:"requires_futures"`
  SessionRoles    interface{} `json:"session_roles"`
  SortOrder       *int32      `json:"sort_order"`
  UpdatedAt       *string     `json:"updated_at"`
}

type PublicStrategyRegistryUpdate struct {
  Category        *string     `json:"category"`
  CreatedAt       *string     `json:"created_at"`
  CrossExchange   *bool       `json:"cross_exchange"`
  Defaults        interface{} `json:"defaults"`
  Description     *string     `json:"description"`
  DisplayName     *string     `json:"display_name"`
  Enabled         *bool       `json:"enabled"`
  Exchanges       interface{} `json:"exchanges"`
  Fields          interface{} `json:"fields"`
  Id              *string     `json:"id"`
  LiveOnly        *bool       `json:"live_only"`
  RequiresFutures *bool       `json:"requires_futures"`
  SessionRoles    interface{} `json:"session_roles"`
  SortOrder       *int32      `json:"sort_order"`
  UpdatedAt       *string     `json:"updated_at"`
}

type PublicStrategyInstancesSelect struct {
  Config        interface{} `json:"config"`
  CreatedAt     string      `json:"created_at"`
  CrossExchange bool        `json:"cross_exchange"`
  Exchange      string      `json:"exchange"`
  InstanceId    string      `json:"instance_id"`
  Mode          string      `json:"mode"`
  Name          string      `json:"name"`
  Sessions      interface{} `json:"sessions"`
  Strategy      string      `json:"strategy"`
  Symbol        string      `json:"symbol"`
  UpdatedAt     string      `json:"updated_at"`
  UserId        string      `json:"user_id"`
}

type PublicStrategyInstancesInsert struct {
  Config        interface{} `json:"config"`
  CreatedAt     *string     `json:"created_at"`
  CrossExchange *bool       `json:"cross_exchange"`
  Exchange      *string     `json:"exchange"`
  InstanceId    string      `json:"instance_id"`
  Mode          string      `json:"mode"`
  Name          *string     `json:"name"`
  Sessions      interface{} `json:"sessions"`
  Strategy      string      `json:"strategy"`
  Symbol        *string     `json:"symbol"`
  UpdatedAt     *string     `json:"updated_at"`
  UserId        string      `json:"user_id"`
}

type PublicStrategyInstancesUpdate struct {
  Config        interface{} `json:"config"`
  CreatedAt     *string     `json:"created_at"`
  CrossExchange *bool       `json:"cross_exchange"`
  Exchange      *string     `json:"exchange"`
  InstanceId    *string     `json:"instance_id"`
  Mode          *string     `json:"mode"`
  Name          *string     `json:"name"`
  Sessions      interface{} `json:"sessions"`
  Strategy      *string     `json:"strategy"`
  Symbol        *string     `json:"symbol"`
  UpdatedAt     *string     `json:"updated_at"`
  UserId        *string     `json:"user_id"`
}

type PublicPaperOrdersSelect struct {
  ActualOrderId      int64   `json:"actual_order_id"`
  ClientOrderId      string  `json:"client_order_id"`
  CreatedAt          string  `json:"created_at"`
  Exchange           string  `json:"exchange"`
  ExecutedQuantity   *string `json:"executed_quantity"`
  Id                 string  `json:"id"`
  IsFutures          bool    `json:"is_futures"`
  IsIsolated         bool    `json:"is_isolated"`
  IsMargin           bool    `json:"is_margin"`
  IsWorking          bool    `json:"is_working"`
  OrderId            string  `json:"order_id"`
  OrderType          string  `json:"order_type"`
  OrderUuid          string  `json:"order_uuid"`
  Price              string  `json:"price"`
  Quantity           string  `json:"quantity"`
  Side               string  `json:"side"`
  Status             string  `json:"status"`
  StopPrice          string  `json:"stop_price"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TimeInForce        string  `json:"time_in_force"`
  UpdatedAt          string  `json:"updated_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperOrdersInsert struct {
  ActualOrderId      *int64  `json:"actual_order_id"`
  ClientOrderId      *string `json:"client_order_id"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  ExecutedQuantity   *string `json:"executed_quantity"`
  Id                 *string `json:"id"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMargin           *bool   `json:"is_margin"`
  IsWorking          *bool   `json:"is_working"`
  OrderId            *string `json:"order_id"`
  OrderType          *string `json:"order_type"`
  OrderUuid          *string `json:"order_uuid"`
  Price              *string `json:"price"`
  Quantity           *string `json:"quantity"`
  Side               *string `json:"side"`
  Status             *string `json:"status"`
  StopPrice          *string `json:"stop_price"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TimeInForce        *string `json:"time_in_force"`
  UpdatedAt          *string `json:"updated_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperOrdersUpdate struct {
  ActualOrderId      *int64  `json:"actual_order_id"`
  ClientOrderId      *string `json:"client_order_id"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  ExecutedQuantity   *string `json:"executed_quantity"`
  Id                 *string `json:"id"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMargin           *bool   `json:"is_margin"`
  IsWorking          *bool   `json:"is_working"`
  OrderId            *string `json:"order_id"`
  OrderType          *string `json:"order_type"`
  OrderUuid          *string `json:"order_uuid"`
  Price              *string `json:"price"`
  Quantity           *string `json:"quantity"`
  Side               *string `json:"side"`
  Status             *string `json:"status"`
  StopPrice          *string `json:"stop_price"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TimeInForce        *string `json:"time_in_force"`
  UpdatedAt          *string `json:"updated_at"`
  UserId             *string `json:"user_id"`
}

type PublicPaperTradesSelect struct {
  Exchange           string  `json:"exchange"`
  Fee                string  `json:"fee"`
  FeeCurrency        string  `json:"fee_currency"`
  Id                 string  `json:"id"`
  IsBuyer            bool    `json:"is_buyer"`
  IsFutures          bool    `json:"is_futures"`
  IsIsolated         bool    `json:"is_isolated"`
  IsMaker            bool    `json:"is_maker"`
  IsMargin           bool    `json:"is_margin"`
  OrderId            string  `json:"order_id"`
  OrderUuid          string  `json:"order_uuid"`
  Pnl                *string `json:"pnl"`
  Price              string  `json:"price"`
  Quantity           string  `json:"quantity"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               string  `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            string  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperTradesInsert struct {
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  OrderId            *string `json:"order_id"`
  OrderUuid          *string `json:"order_uuid"`
  Pnl                *string `json:"pnl"`
  Price              *string `json:"price"`
  Quantity           *string `json:"quantity"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *string `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperTradesUpdate struct {
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  OrderId            *string `json:"order_id"`
  OrderUuid          *string `json:"order_uuid"`
  Pnl                *string `json:"pnl"`
  Price              *string `json:"price"`
  Quantity           *string `json:"quantity"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *string `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             *string `json:"user_id"`
}

type PublicPaperPositionsSelect struct {
  AverageCost        string  `json:"average_cost"`
  Base               string  `json:"base"`
  BaseCurrency       string  `json:"base_currency"`
  CreatedAt          string  `json:"created_at"`
  Exchange           string  `json:"exchange"`
  Id                 string  `json:"id"`
  NetProfit          *string `json:"net_profit"`
  Profit             *string `json:"profit"`
  Quote              string  `json:"quote"`
  QuoteCurrency      string  `json:"quote_currency"`
  Side               string  `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperPositionsInsert struct {
  AverageCost        *string `json:"average_cost"`
  Base               *string `json:"base"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Id                 *string `json:"id"`
  NetProfit          *string `json:"net_profit"`
  Profit             *string `json:"profit"`
  Quote              *string `json:"quote"`
  QuoteCurrency      *string `json:"quote_currency"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperPositionsUpdate struct {
  AverageCost        *string `json:"average_cost"`
  Base               *string `json:"base"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Id                 *string `json:"id"`
  NetProfit          *string `json:"net_profit"`
  Profit             *string `json:"profit"`
  Quote              *string `json:"quote"`
  QuoteCurrency      *string `json:"quote_currency"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *int64  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             *string `json:"user_id"`
}

type PublicPaperProfitsSelect struct {
  AverageCost        string  `json:"average_cost"`
  BaseCurrency       string  `json:"base_currency"`
  CreatedAt          string  `json:"created_at"`
  Exchange           string  `json:"exchange"`
  Fee                string  `json:"fee"`
  FeeCurrency        string  `json:"fee_currency"`
  FeeInUsd           *string `json:"fee_in_usd"`
  Id                 string  `json:"id"`
  IsBuyer            bool    `json:"is_buyer"`
  IsFutures          bool    `json:"is_futures"`
  IsIsolated         bool    `json:"is_isolated"`
  IsMaker            bool    `json:"is_maker"`
  IsMargin           bool    `json:"is_margin"`
  NetProfit          string  `json:"net_profit"`
  NetProfitMargin    string  `json:"net_profit_margin"`
  Price              string  `json:"price"`
  Profit             string  `json:"profit"`
  ProfitMargin       string  `json:"profit_margin"`
  Quantity           string  `json:"quantity"`
  QuoteCurrency      string  `json:"quote_currency"`
  QuoteQuantity      string  `json:"quote_quantity"`
  Side               string  `json:"side"`
  Strategy           string  `json:"strategy"`
  StrategyInstanceId string  `json:"strategy_instance_id"`
  Symbol             string  `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperProfitsInsert struct {
  AverageCost        *string `json:"average_cost"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  FeeInUsd           *string `json:"fee_in_usd"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  NetProfit          *string `json:"net_profit"`
  NetProfitMargin    *string `json:"net_profit_margin"`
  Price              *string `json:"price"`
  Profit             *string `json:"profit"`
  ProfitMargin       *string `json:"profit_margin"`
  Quantity           *string `json:"quantity"`
  QuoteCurrency      *string `json:"quote_currency"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            int64   `json:"trade_id"`
  TradedAt           string  `json:"traded_at"`
  UserId             string  `json:"user_id"`
}

type PublicPaperProfitsUpdate struct {
  AverageCost        *string `json:"average_cost"`
  BaseCurrency       *string `json:"base_currency"`
  CreatedAt          *string `json:"created_at"`
  Exchange           *string `json:"exchange"`
  Fee                *string `json:"fee"`
  FeeCurrency        *string `json:"fee_currency"`
  FeeInUsd           *string `json:"fee_in_usd"`
  Id                 *string `json:"id"`
  IsBuyer            *bool   `json:"is_buyer"`
  IsFutures          *bool   `json:"is_futures"`
  IsIsolated         *bool   `json:"is_isolated"`
  IsMaker            *bool   `json:"is_maker"`
  IsMargin           *bool   `json:"is_margin"`
  NetProfit          *string `json:"net_profit"`
  NetProfitMargin    *string `json:"net_profit_margin"`
  Price              *string `json:"price"`
  Profit             *string `json:"profit"`
  ProfitMargin       *string `json:"profit_margin"`
  Quantity           *string `json:"quantity"`
  QuoteCurrency      *string `json:"quote_currency"`
  QuoteQuantity      *string `json:"quote_quantity"`
  Side               *string `json:"side"`
  Strategy           *string `json:"strategy"`
  StrategyInstanceId *string `json:"strategy_instance_id"`
  Symbol             *string `json:"symbol"`
  TradeId            *int64  `json:"trade_id"`
  TradedAt           *string `json:"traded_at"`
  UserId             *string `json:"user_id"`
}

type PublicNavHistoryDetailsSelect struct {
  Available      string `json:"available"`
  Balance        string `json:"balance"`
  Borrowed       string `json:"borrowed"`
  Currency       string `json:"currency"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  Interest       string `json:"interest"`
  IsIsolated     bool   `json:"is_isolated"`
  IsMargin       bool   `json:"is_margin"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Locked         string `json:"locked"`
  NetAsset       string `json:"net_asset"`
  NetAssetInBtc  string `json:"net_asset_in_btc"`
  NetAssetInUsd  string `json:"net_asset_in_usd"`
  PriceInUsd     string `json:"price_in_usd"`
  Session        string `json:"session"`
  Subaccount     string `json:"subaccount"`
  Time           string `json:"time"`
  UserId         string `json:"user_id"`
}

type PublicNavHistoryDetailsInsert struct {
  Available      *string `json:"available"`
  Balance        *string `json:"balance"`
  Borrowed       *string `json:"borrowed"`
  Currency       *string `json:"currency"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  IsIsolated     *bool   `json:"is_isolated"`
  IsMargin       *bool   `json:"is_margin"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Locked         *string `json:"locked"`
  NetAsset       *string `json:"net_asset"`
  NetAssetInBtc  *string `json:"net_asset_in_btc"`
  NetAssetInUsd  *string `json:"net_asset_in_usd"`
  PriceInUsd     *string `json:"price_in_usd"`
  Session        *string `json:"session"`
  Subaccount     *string `json:"subaccount"`
  Time           *string `json:"time"`
  UserId         string  `json:"user_id"`
}

type PublicNavHistoryDetailsUpdate struct {
  Available      *string `json:"available"`
  Balance        *string `json:"balance"`
  Borrowed       *string `json:"borrowed"`
  Currency       *string `json:"currency"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  IsIsolated     *bool   `json:"is_isolated"`
  IsMargin       *bool   `json:"is_margin"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Locked         *string `json:"locked"`
  NetAsset       *string `json:"net_asset"`
  NetAssetInBtc  *string `json:"net_asset_in_btc"`
  NetAssetInUsd  *string `json:"net_asset_in_usd"`
  PriceInUsd     *string `json:"price_in_usd"`
  Session        *string `json:"session"`
  Subaccount     *string `json:"subaccount"`
  Time           *string `json:"time"`
  UserId         *string `json:"user_id"`
}

type PublicRewardsSelect struct {
  CreatedAt  string  `json:"created_at"`
  Currency   string  `json:"currency"`
  Exchange   string  `json:"exchange"`
  Id         string  `json:"id"`
  Note       *string `json:"note"`
  Quantity   string  `json:"quantity"`
  RewardType string  `json:"reward_type"`
  Spent      bool    `json:"spent"`
  State      string  `json:"state"`
  UserId     string  `json:"user_id"`
  Uuid       string  `json:"uuid"`
}

type PublicRewardsInsert struct {
  CreatedAt  *string `json:"created_at"`
  Currency   *string `json:"currency"`
  Exchange   *string `json:"exchange"`
  Id         *string `json:"id"`
  Note       *string `json:"note"`
  Quantity   *string `json:"quantity"`
  RewardType *string `json:"reward_type"`
  Spent      *bool   `json:"spent"`
  State      *string `json:"state"`
  UserId     string  `json:"user_id"`
  Uuid       *string `json:"uuid"`
}

type PublicRewardsUpdate struct {
  CreatedAt  *string `json:"created_at"`
  Currency   *string `json:"currency"`
  Exchange   *string `json:"exchange"`
  Id         *string `json:"id"`
  Note       *string `json:"note"`
  Quantity   *string `json:"quantity"`
  RewardType *string `json:"reward_type"`
  Spent      *bool   `json:"spent"`
  State      *string `json:"state"`
  UserId     *string `json:"user_id"`
  Uuid       *string `json:"uuid"`
}

type PublicWithdrawsSelect struct {
  Address        string `json:"address"`
  Amount         string `json:"amount"`
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  Network        string `json:"network"`
  Time           string `json:"time"`
  TxnFee         string `json:"txn_fee"`
  TxnFeeCurrency string `json:"txn_fee_currency"`
  TxnId          string `json:"txn_id"`
  UserId         string `json:"user_id"`
}

type PublicWithdrawsInsert struct {
  Address        *string `json:"address"`
  Amount         *string `json:"amount"`
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Network        *string `json:"network"`
  Time           *string `json:"time"`
  TxnFee         *string `json:"txn_fee"`
  TxnFeeCurrency *string `json:"txn_fee_currency"`
  TxnId          *string `json:"txn_id"`
  UserId         string  `json:"user_id"`
}

type PublicWithdrawsUpdate struct {
  Address        *string `json:"address"`
  Amount         *string `json:"amount"`
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Network        *string `json:"network"`
  Time           *string `json:"time"`
  TxnFee         *string `json:"txn_fee"`
  TxnFeeCurrency *string `json:"txn_fee_currency"`
  TxnId          *string `json:"txn_id"`
  UserId         *string `json:"user_id"`
}

type PublicDepositsSelect struct {
  Address  string `json:"address"`
  Amount   string `json:"amount"`
  Asset    string `json:"asset"`
  Exchange string `json:"exchange"`
  Id       string `json:"id"`
  Time     string `json:"time"`
  TxnId    string `json:"txn_id"`
  UserId   string `json:"user_id"`
}

type PublicDepositsInsert struct {
  Address  *string `json:"address"`
  Amount   *string `json:"amount"`
  Asset    *string `json:"asset"`
  Exchange *string `json:"exchange"`
  Id       *string `json:"id"`
  Time     *string `json:"time"`
  TxnId    *string `json:"txn_id"`
  UserId   string  `json:"user_id"`
}

type PublicDepositsUpdate struct {
  Address  *string `json:"address"`
  Amount   *string `json:"amount"`
  Asset    *string `json:"asset"`
  Exchange *string `json:"exchange"`
  Id       *string `json:"id"`
  Time     *string `json:"time"`
  TxnId    *string `json:"txn_id"`
  UserId   *string `json:"user_id"`
}

type PublicMarginLoansSelect struct {
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Principle      string `json:"principle"`
  Time           string `json:"time"`
  TransactionId  int64  `json:"transaction_id"`
  UserId         string `json:"user_id"`
}

type PublicMarginLoansInsert struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         string  `json:"user_id"`
}

type PublicMarginLoansUpdate struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         *string `json:"user_id"`
}

type PublicMarginRepaysSelect struct {
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Principle      string `json:"principle"`
  Time           string `json:"time"`
  TransactionId  int64  `json:"transaction_id"`
  UserId         string `json:"user_id"`
}

type PublicMarginRepaysInsert struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         string  `json:"user_id"`
}

type PublicMarginRepaysUpdate struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         *string `json:"user_id"`
}

type PublicMarginInterestsSelect struct {
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  Interest       string `json:"interest"`
  InterestRate   string `json:"interest_rate"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Principle      string `json:"principle"`
  Time           string `json:"time"`
  UserId         string `json:"user_id"`
}

type PublicMarginInterestsInsert struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  InterestRate   *string `json:"interest_rate"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  UserId         string  `json:"user_id"`
}

type PublicMarginInterestsUpdate struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  InterestRate   *string `json:"interest_rate"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  UserId         *string `json:"user_id"`
}

type PublicMarginLiquidationsSelect struct {
  AveragePrice     string `json:"average_price"`
  Exchange         string `json:"exchange"`
  ExecutedQuantity string `json:"executed_quantity"`
  Id               string `json:"id"`
  IsIsolated       bool   `json:"is_isolated"`
  OrderId          int64  `json:"order_id"`
  Price            string `json:"price"`
  Quantity         string `json:"quantity"`
  Side             string `json:"side"`
  Symbol           string `json:"symbol"`
  Time             string `json:"time"`
  TimeInForce      string `json:"time_in_force"`
  UserId           string `json:"user_id"`
}

type PublicMarginLiquidationsInsert struct {
  AveragePrice     *string `json:"average_price"`
  Exchange         *string `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               *string `json:"id"`
  IsIsolated       *bool   `json:"is_isolated"`
  OrderId          *int64  `json:"order_id"`
  Price            *string `json:"price"`
  Quantity         *string `json:"quantity"`
  Side             *string `json:"side"`
  Symbol           *string `json:"symbol"`
  Time             *string `json:"time"`
  TimeInForce      *string `json:"time_in_force"`
  UserId           string  `json:"user_id"`
}

type PublicMarginLiquidationsUpdate struct {
  AveragePrice     *string `json:"average_price"`
  Exchange         *string `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               *string `json:"id"`
  IsIsolated       *bool   `json:"is_isolated"`
  OrderId          *int64  `json:"order_id"`
  Price            *string `json:"price"`
  Quantity         *string `json:"quantity"`
  Side             *string `json:"side"`
  Symbol           *string `json:"symbol"`
  Time             *string `json:"time"`
  TimeInForce      *string `json:"time_in_force"`
  UserId           *string `json:"user_id"`
}

type PublicFuturesPositionRisksSelect struct {
  Adl                    string `json:"adl"`
  BreakEvenPrice         string `json:"break_even_price"`
  EntryPrice             string `json:"entry_price"`
  Exchange               string `json:"exchange"`
  Id                     string `json:"id"`
  InitialMargin          string `json:"initial_margin"`
  Leverage               string `json:"leverage"`
  LiquidationPrice       string `json:"liquidation_price"`
  MaintMargin            string `json:"maint_margin"`
  MarginAsset            string `json:"margin_asset"`
  MarkPrice              string `json:"mark_price"`
  Notional               string `json:"notional"`
  OpenOrderInitialMargin string `json:"open_order_initial_margin"`
  PositionAmount         string `json:"position_amount"`
  PositionInitialMargin  string `json:"position_initial_margin"`
  PositionSide           string `json:"position_side"`
  Symbol                 string `json:"symbol"`
  UnrealizedPnl          string `json:"unrealized_pnl"`
  UpdatedAt              string `json:"updated_at"`
  UserId                 string `json:"user_id"`
}

type PublicFuturesPositionRisksInsert struct {
  Adl                    *string `json:"adl"`
  BreakEvenPrice         *string `json:"break_even_price"`
  EntryPrice             *string `json:"entry_price"`
  Exchange               *string `json:"exchange"`
  Id                     *string `json:"id"`
  InitialMargin          *string `json:"initial_margin"`
  Leverage               *string `json:"leverage"`
  LiquidationPrice       *string `json:"liquidation_price"`
  MaintMargin            *string `json:"maint_margin"`
  MarginAsset            *string `json:"margin_asset"`
  MarkPrice              *string `json:"mark_price"`
  Notional               *string `json:"notional"`
  OpenOrderInitialMargin *string `json:"open_order_initial_margin"`
  PositionAmount         *string `json:"position_amount"`
  PositionInitialMargin  *string `json:"position_initial_margin"`
  PositionSide           *string `json:"position_side"`
  Symbol                 *string `json:"symbol"`
  UnrealizedPnl          *string `json:"unrealized_pnl"`
  UpdatedAt              *string `json:"updated_at"`
  UserId                 string  `json:"user_id"`
}

type PublicFuturesPositionRisksUpdate struct {
  Adl                    *string `json:"adl"`
  BreakEvenPrice         *string `json:"break_even_price"`
  EntryPrice             *string `json:"entry_price"`
  Exchange               *string `json:"exchange"`
  Id                     *string `json:"id"`
  InitialMargin          *string `json:"initial_margin"`
  Leverage               *string `json:"leverage"`
  LiquidationPrice       *string `json:"liquidation_price"`
  MaintMargin            *string `json:"maint_margin"`
  MarginAsset            *string `json:"margin_asset"`
  MarkPrice              *string `json:"mark_price"`
  Notional               *string `json:"notional"`
  OpenOrderInitialMargin *string `json:"open_order_initial_margin"`
  PositionAmount         *string `json:"position_amount"`
  PositionInitialMargin  *string `json:"position_initial_margin"`
  PositionSide           *string `json:"position_side"`
  Symbol                 *string `json:"symbol"`
  UnrealizedPnl          *string `json:"unrealized_pnl"`
  UpdatedAt              *string `json:"updated_at"`
  UserId                 *string `json:"user_id"`
}

type PublicPaperNavHistoryDetailsSelect struct {
  Available      string `json:"available"`
  Balance        string `json:"balance"`
  Borrowed       string `json:"borrowed"`
  Currency       string `json:"currency"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  Interest       string `json:"interest"`
  IsIsolated     bool   `json:"is_isolated"`
  IsMargin       bool   `json:"is_margin"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Locked         string `json:"locked"`
  NetAsset       string `json:"net_asset"`
  NetAssetInBtc  string `json:"net_asset_in_btc"`
  NetAssetInUsd  string `json:"net_asset_in_usd"`
  PriceInUsd     string `json:"price_in_usd"`
  Session        string `json:"session"`
  Subaccount     string `json:"subaccount"`
  Time           string `json:"time"`
  UserId         string `json:"user_id"`
}

type PublicPaperNavHistoryDetailsInsert struct {
  Available      *string `json:"available"`
  Balance        *string `json:"balance"`
  Borrowed       *string `json:"borrowed"`
  Currency       *string `json:"currency"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  IsIsolated     *bool   `json:"is_isolated"`
  IsMargin       *bool   `json:"is_margin"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Locked         *string `json:"locked"`
  NetAsset       *string `json:"net_asset"`
  NetAssetInBtc  *string `json:"net_asset_in_btc"`
  NetAssetInUsd  *string `json:"net_asset_in_usd"`
  PriceInUsd     *string `json:"price_in_usd"`
  Session        *string `json:"session"`
  Subaccount     *string `json:"subaccount"`
  Time           *string `json:"time"`
  UserId         string  `json:"user_id"`
}

type PublicPaperNavHistoryDetailsUpdate struct {
  Available      *string `json:"available"`
  Balance        *string `json:"balance"`
  Borrowed       *string `json:"borrowed"`
  Currency       *string `json:"currency"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  IsIsolated     *bool   `json:"is_isolated"`
  IsMargin       *bool   `json:"is_margin"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Locked         *string `json:"locked"`
  NetAsset       *string `json:"net_asset"`
  NetAssetInBtc  *string `json:"net_asset_in_btc"`
  NetAssetInUsd  *string `json:"net_asset_in_usd"`
  PriceInUsd     *string `json:"price_in_usd"`
  Session        *string `json:"session"`
  Subaccount     *string `json:"subaccount"`
  Time           *string `json:"time"`
  UserId         *string `json:"user_id"`
}

type PublicPaperRewardsSelect struct {
  CreatedAt  string  `json:"created_at"`
  Currency   string  `json:"currency"`
  Exchange   string  `json:"exchange"`
  Id         string  `json:"id"`
  Note       *string `json:"note"`
  Quantity   string  `json:"quantity"`
  RewardType string  `json:"reward_type"`
  Spent      bool    `json:"spent"`
  State      string  `json:"state"`
  UserId     string  `json:"user_id"`
  Uuid       string  `json:"uuid"`
}

type PublicPaperRewardsInsert struct {
  CreatedAt  *string `json:"created_at"`
  Currency   *string `json:"currency"`
  Exchange   *string `json:"exchange"`
  Id         *string `json:"id"`
  Note       *string `json:"note"`
  Quantity   *string `json:"quantity"`
  RewardType *string `json:"reward_type"`
  Spent      *bool   `json:"spent"`
  State      *string `json:"state"`
  UserId     string  `json:"user_id"`
  Uuid       *string `json:"uuid"`
}

type PublicPaperRewardsUpdate struct {
  CreatedAt  *string `json:"created_at"`
  Currency   *string `json:"currency"`
  Exchange   *string `json:"exchange"`
  Id         *string `json:"id"`
  Note       *string `json:"note"`
  Quantity   *string `json:"quantity"`
  RewardType *string `json:"reward_type"`
  Spent      *bool   `json:"spent"`
  State      *string `json:"state"`
  UserId     *string `json:"user_id"`
  Uuid       *string `json:"uuid"`
}

type PublicPaperWithdrawsSelect struct {
  Address        string `json:"address"`
  Amount         string `json:"amount"`
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  Network        string `json:"network"`
  Time           string `json:"time"`
  TxnFee         string `json:"txn_fee"`
  TxnFeeCurrency string `json:"txn_fee_currency"`
  TxnId          string `json:"txn_id"`
  UserId         string `json:"user_id"`
}

type PublicPaperWithdrawsInsert struct {
  Address        *string `json:"address"`
  Amount         *string `json:"amount"`
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Network        *string `json:"network"`
  Time           *string `json:"time"`
  TxnFee         *string `json:"txn_fee"`
  TxnFeeCurrency *string `json:"txn_fee_currency"`
  TxnId          *string `json:"txn_id"`
  UserId         string  `json:"user_id"`
}

type PublicPaperWithdrawsUpdate struct {
  Address        *string `json:"address"`
  Amount         *string `json:"amount"`
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Network        *string `json:"network"`
  Time           *string `json:"time"`
  TxnFee         *string `json:"txn_fee"`
  TxnFeeCurrency *string `json:"txn_fee_currency"`
  TxnId          *string `json:"txn_id"`
  UserId         *string `json:"user_id"`
}

type PublicPaperDepositsSelect struct {
  Address  string `json:"address"`
  Amount   string `json:"amount"`
  Asset    string `json:"asset"`
  Exchange string `json:"exchange"`
  Id       string `json:"id"`
  Time     string `json:"time"`
  TxnId    string `json:"txn_id"`
  UserId   string `json:"user_id"`
}

type PublicPaperDepositsInsert struct {
  Address  *string `json:"address"`
  Amount   *string `json:"amount"`
  Asset    *string `json:"asset"`
  Exchange *string `json:"exchange"`
  Id       *string `json:"id"`
  Time     *string `json:"time"`
  TxnId    *string `json:"txn_id"`
  UserId   string  `json:"user_id"`
}

type PublicPaperDepositsUpdate struct {
  Address  *string `json:"address"`
  Amount   *string `json:"amount"`
  Asset    *string `json:"asset"`
  Exchange *string `json:"exchange"`
  Id       *string `json:"id"`
  Time     *string `json:"time"`
  TxnId    *string `json:"txn_id"`
  UserId   *string `json:"user_id"`
}

type PublicPaperMarginLoansSelect struct {
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Principle      string `json:"principle"`
  Time           string `json:"time"`
  TransactionId  int64  `json:"transaction_id"`
  UserId         string `json:"user_id"`
}

type PublicPaperMarginLoansInsert struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         string  `json:"user_id"`
}

type PublicPaperMarginLoansUpdate struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         *string `json:"user_id"`
}

type PublicPaperMarginRepaysSelect struct {
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Principle      string `json:"principle"`
  Time           string `json:"time"`
  TransactionId  int64  `json:"transaction_id"`
  UserId         string `json:"user_id"`
}

type PublicPaperMarginRepaysInsert struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         string  `json:"user_id"`
}

type PublicPaperMarginRepaysUpdate struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  TransactionId  *int64  `json:"transaction_id"`
  UserId         *string `json:"user_id"`
}

type PublicPaperMarginInterestsSelect struct {
  Asset          string `json:"asset"`
  Exchange       string `json:"exchange"`
  Id             string `json:"id"`
  Interest       string `json:"interest"`
  InterestRate   string `json:"interest_rate"`
  IsolatedSymbol string `json:"isolated_symbol"`
  Principle      string `json:"principle"`
  Time           string `json:"time"`
  UserId         string `json:"user_id"`
}

type PublicPaperMarginInterestsInsert struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  InterestRate   *string `json:"interest_rate"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  UserId         string  `json:"user_id"`
}

type PublicPaperMarginInterestsUpdate struct {
  Asset          *string `json:"asset"`
  Exchange       *string `json:"exchange"`
  Id             *string `json:"id"`
  Interest       *string `json:"interest"`
  InterestRate   *string `json:"interest_rate"`
  IsolatedSymbol *string `json:"isolated_symbol"`
  Principle      *string `json:"principle"`
  Time           *string `json:"time"`
  UserId         *string `json:"user_id"`
}

type PublicPaperMarginLiquidationsSelect struct {
  AveragePrice     string `json:"average_price"`
  Exchange         string `json:"exchange"`
  ExecutedQuantity string `json:"executed_quantity"`
  Id               string `json:"id"`
  IsIsolated       bool   `json:"is_isolated"`
  OrderId          int64  `json:"order_id"`
  Price            string `json:"price"`
  Quantity         string `json:"quantity"`
  Side             string `json:"side"`
  Symbol           string `json:"symbol"`
  Time             string `json:"time"`
  TimeInForce      string `json:"time_in_force"`
  UserId           string `json:"user_id"`
}

type PublicPaperMarginLiquidationsInsert struct {
  AveragePrice     *string `json:"average_price"`
  Exchange         *string `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               *string `json:"id"`
  IsIsolated       *bool   `json:"is_isolated"`
  OrderId          *int64  `json:"order_id"`
  Price            *string `json:"price"`
  Quantity         *string `json:"quantity"`
  Side             *string `json:"side"`
  Symbol           *string `json:"symbol"`
  Time             *string `json:"time"`
  TimeInForce      *string `json:"time_in_force"`
  UserId           string  `json:"user_id"`
}

type PublicPaperMarginLiquidationsUpdate struct {
  AveragePrice     *string `json:"average_price"`
  Exchange         *string `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               *string `json:"id"`
  IsIsolated       *bool   `json:"is_isolated"`
  OrderId          *int64  `json:"order_id"`
  Price            *string `json:"price"`
  Quantity         *string `json:"quantity"`
  Side             *string `json:"side"`
  Symbol           *string `json:"symbol"`
  Time             *string `json:"time"`
  TimeInForce      *string `json:"time_in_force"`
  UserId           *string `json:"user_id"`
}

type PublicPaperFuturesPositionRisksSelect struct {
  Adl                    string `json:"adl"`
  BreakEvenPrice         string `json:"break_even_price"`
  EntryPrice             string `json:"entry_price"`
  Exchange               string `json:"exchange"`
  Id                     string `json:"id"`
  InitialMargin          string `json:"initial_margin"`
  Leverage               string `json:"leverage"`
  LiquidationPrice       string `json:"liquidation_price"`
  MaintMargin            string `json:"maint_margin"`
  MarginAsset            string `json:"margin_asset"`
  MarkPrice              string `json:"mark_price"`
  Notional               string `json:"notional"`
  OpenOrderInitialMargin string `json:"open_order_initial_margin"`
  PositionAmount         string `json:"position_amount"`
  PositionInitialMargin  string `json:"position_initial_margin"`
  PositionSide           string `json:"position_side"`
  Symbol                 string `json:"symbol"`
  UnrealizedPnl          string `json:"unrealized_pnl"`
  UpdatedAt              string `json:"updated_at"`
  UserId                 string `json:"user_id"`
}

type PublicPaperFuturesPositionRisksInsert struct {
  Adl                    *string `json:"adl"`
  BreakEvenPrice         *string `json:"break_even_price"`
  EntryPrice             *string `json:"entry_price"`
  Exchange               *string `json:"exchange"`
  Id                     *string `json:"id"`
  InitialMargin          *string `json:"initial_margin"`
  Leverage               *string `json:"leverage"`
  LiquidationPrice       *string `json:"liquidation_price"`
  MaintMargin            *string `json:"maint_margin"`
  MarginAsset            *string `json:"margin_asset"`
  MarkPrice              *string `json:"mark_price"`
  Notional               *string `json:"notional"`
  OpenOrderInitialMargin *string `json:"open_order_initial_margin"`
  PositionAmount         *string `json:"position_amount"`
  PositionInitialMargin  *string `json:"position_initial_margin"`
  PositionSide           *string `json:"position_side"`
  Symbol                 *string `json:"symbol"`
  UnrealizedPnl          *string `json:"unrealized_pnl"`
  UpdatedAt              *string `json:"updated_at"`
  UserId                 string  `json:"user_id"`
}

type PublicPaperFuturesPositionRisksUpdate struct {
  Adl                    *string `json:"adl"`
  BreakEvenPrice         *string `json:"break_even_price"`
  EntryPrice             *string `json:"entry_price"`
  Exchange               *string `json:"exchange"`
  Id                     *string `json:"id"`
  InitialMargin          *string `json:"initial_margin"`
  Leverage               *string `json:"leverage"`
  LiquidationPrice       *string `json:"liquidation_price"`
  MaintMargin            *string `json:"maint_margin"`
  MarginAsset            *string `json:"margin_asset"`
  MarkPrice              *string `json:"mark_price"`
  Notional               *string `json:"notional"`
  OpenOrderInitialMargin *string `json:"open_order_initial_margin"`
  PositionAmount         *string `json:"position_amount"`
  PositionInitialMargin  *string `json:"position_initial_margin"`
  PositionSide           *string `json:"position_side"`
  Symbol                 *string `json:"symbol"`
  UnrealizedPnl          *string `json:"unrealized_pnl"`
  UpdatedAt              *string `json:"updated_at"`
  UserId                 *string `json:"user_id"`
}
