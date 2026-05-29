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

type PublicBotsSelect struct {
  BbgoPid       *int32      `json:"bbgo_pid"`
  Config        interface{} `json:"config"`
  ConfigPath    *string     `json:"config_path"`
  CreatedAt     string      `json:"created_at"`
  Exchange      string      `json:"exchange"`
  GrpcPort      *int32      `json:"grpc_port"`
  Id            string      `json:"id"`
  Mode          string      `json:"mode"`
  Name          string      `json:"name"`
  Status        string      `json:"status"`
  Strategy      string      `json:"strategy"`
  UpdatedAt     string      `json:"updated_at"`
  UserId        string      `json:"user_id"`
  WebserverPort *int32      `json:"webserver_port"`
}

type PublicBotsInsert struct {
  BbgoPid       *int32      `json:"bbgo_pid"`
  Config        interface{} `json:"config"`
  ConfigPath    *string     `json:"config_path"`
  CreatedAt     *string     `json:"created_at"`
  Exchange      string      `json:"exchange"`
  GrpcPort      *int32      `json:"grpc_port"`
  Id            *string     `json:"id"`
  Mode          *string     `json:"mode"`
  Name          string      `json:"name"`
  Status        *string     `json:"status"`
  Strategy      string      `json:"strategy"`
  UpdatedAt     *string     `json:"updated_at"`
  UserId        string      `json:"user_id"`
  WebserverPort *int32      `json:"webserver_port"`
}

type PublicBotsUpdate struct {
  BbgoPid       *int32      `json:"bbgo_pid"`
  Config        interface{} `json:"config"`
  ConfigPath    *string     `json:"config_path"`
  CreatedAt     *string     `json:"created_at"`
  Exchange      *string     `json:"exchange"`
  GrpcPort      *int32      `json:"grpc_port"`
  Id            *string     `json:"id"`
  Mode          *string     `json:"mode"`
  Name          *string     `json:"name"`
  Status        *string     `json:"status"`
  Strategy      *string     `json:"strategy"`
  UpdatedAt     *string     `json:"updated_at"`
  UserId        *string     `json:"user_id"`
  WebserverPort *int32      `json:"webserver_port"`
}

type PublicSyncOrdersSelect struct {
  ActualOrderId    int64   `json:"actual_order_id"`
  BotId            *string `json:"bot_id"`
  ClientOrderId    string  `json:"client_order_id"`
  CreatedAt        string  `json:"created_at"`
  CreationTime     *string `json:"creation_time"`
  Exchange         string  `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               string  `json:"id"`
  IsFutures        bool    `json:"is_futures"`
  IsIsolated       bool    `json:"is_isolated"`
  IsMargin         bool    `json:"is_margin"`
  IsWorking        bool    `json:"is_working"`
  OrderId          string  `json:"order_id"`
  OrderUuid        string  `json:"order_uuid"`
  Price            string  `json:"price"`
  Quantity         string  `json:"quantity"`
  Side             string  `json:"side"`
  Status           string  `json:"status"`
  StopPrice        string  `json:"stop_price"`
  Symbol           string  `json:"symbol"`
  SyncedAt         string  `json:"synced_at"`
  TimeInForce      string  `json:"time_in_force"`
  Type             string  `json:"type"`
  UpdatedAt        string  `json:"updated_at"`
  UserId           string  `json:"user_id"`
}

type PublicSyncOrdersInsert struct {
  ActualOrderId    *int64  `json:"actual_order_id"`
  BotId            *string `json:"bot_id"`
  ClientOrderId    *string `json:"client_order_id"`
  CreatedAt        *string `json:"created_at"`
  CreationTime     *string `json:"creation_time"`
  Exchange         *string `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               *string `json:"id"`
  IsFutures        *bool   `json:"is_futures"`
  IsIsolated       *bool   `json:"is_isolated"`
  IsMargin         *bool   `json:"is_margin"`
  IsWorking        *bool   `json:"is_working"`
  OrderId          string  `json:"order_id"`
  OrderUuid        *string `json:"order_uuid"`
  Price            string  `json:"price"`
  Quantity         string  `json:"quantity"`
  Side             string  `json:"side"`
  Status           string  `json:"status"`
  StopPrice        *string `json:"stop_price"`
  Symbol           string  `json:"symbol"`
  SyncedAt         *string `json:"synced_at"`
  TimeInForce      *string `json:"time_in_force"`
  Type             string  `json:"type"`
  UpdatedAt        *string `json:"updated_at"`
  UserId           string  `json:"user_id"`
}

type PublicSyncOrdersUpdate struct {
  ActualOrderId    *int64  `json:"actual_order_id"`
  BotId            *string `json:"bot_id"`
  ClientOrderId    *string `json:"client_order_id"`
  CreatedAt        *string `json:"created_at"`
  CreationTime     *string `json:"creation_time"`
  Exchange         *string `json:"exchange"`
  ExecutedQuantity *string `json:"executed_quantity"`
  Id               *string `json:"id"`
  IsFutures        *bool   `json:"is_futures"`
  IsIsolated       *bool   `json:"is_isolated"`
  IsMargin         *bool   `json:"is_margin"`
  IsWorking        *bool   `json:"is_working"`
  OrderId          *string `json:"order_id"`
  OrderUuid        *string `json:"order_uuid"`
  Price            *string `json:"price"`
  Quantity         *string `json:"quantity"`
  Side             *string `json:"side"`
  Status           *string `json:"status"`
  StopPrice        *string `json:"stop_price"`
  Symbol           *string `json:"symbol"`
  SyncedAt         *string `json:"synced_at"`
  TimeInForce      *string `json:"time_in_force"`
  Type             *string `json:"type"`
  UpdatedAt        *string `json:"updated_at"`
  UserId           *string `json:"user_id"`
}

type PublicSyncTradesSelect struct {
  BotId         *string `json:"bot_id"`
  CreatedAt     string  `json:"created_at"`
  Exchange      string  `json:"exchange"`
  Fee           string  `json:"fee"`
  FeeCurrency   string  `json:"fee_currency"`
  Id            string  `json:"id"`
  IsBuyer       bool    `json:"is_buyer"`
  IsFutures     bool    `json:"is_futures"`
  IsIsolated    bool    `json:"is_isolated"`
  IsMaker       bool    `json:"is_maker"`
  IsMargin      bool    `json:"is_margin"`
  OrderId       string  `json:"order_id"`
  OrderUuid     string  `json:"order_uuid"`
  Pnl           *string `json:"pnl"`
  Price         string  `json:"price"`
  Quantity      string  `json:"quantity"`
  QuoteQuantity *string `json:"quote_quantity"`
  Side          string  `json:"side"`
  Strategy      string  `json:"strategy"`
  Symbol        string  `json:"symbol"`
  SyncedAt      string  `json:"synced_at"`
  TradeId       string  `json:"trade_id"`
  TradedAt      *string `json:"traded_at"`
  UserId        string  `json:"user_id"`
}

type PublicSyncTradesInsert struct {
  BotId         *string `json:"bot_id"`
  CreatedAt     *string `json:"created_at"`
  Exchange      *string `json:"exchange"`
  Fee           string  `json:"fee"`
  FeeCurrency   string  `json:"fee_currency"`
  Id            *string `json:"id"`
  IsBuyer       *bool   `json:"is_buyer"`
  IsFutures     *bool   `json:"is_futures"`
  IsIsolated    *bool   `json:"is_isolated"`
  IsMaker       *bool   `json:"is_maker"`
  IsMargin      *bool   `json:"is_margin"`
  OrderId       string  `json:"order_id"`
  OrderUuid     *string `json:"order_uuid"`
  Pnl           *string `json:"pnl"`
  Price         string  `json:"price"`
  Quantity      string  `json:"quantity"`
  QuoteQuantity *string `json:"quote_quantity"`
  Side          string  `json:"side"`
  Strategy      *string `json:"strategy"`
  Symbol        string  `json:"symbol"`
  SyncedAt      *string `json:"synced_at"`
  TradeId       string  `json:"trade_id"`
  TradedAt      *string `json:"traded_at"`
  UserId        string  `json:"user_id"`
}

type PublicSyncTradesUpdate struct {
  BotId         *string `json:"bot_id"`
  CreatedAt     *string `json:"created_at"`
  Exchange      *string `json:"exchange"`
  Fee           *string `json:"fee"`
  FeeCurrency   *string `json:"fee_currency"`
  Id            *string `json:"id"`
  IsBuyer       *bool   `json:"is_buyer"`
  IsFutures     *bool   `json:"is_futures"`
  IsIsolated    *bool   `json:"is_isolated"`
  IsMaker       *bool   `json:"is_maker"`
  IsMargin      *bool   `json:"is_margin"`
  OrderId       *string `json:"order_id"`
  OrderUuid     *string `json:"order_uuid"`
  Pnl           *string `json:"pnl"`
  Price         *string `json:"price"`
  Quantity      *string `json:"quantity"`
  QuoteQuantity *string `json:"quote_quantity"`
  Side          *string `json:"side"`
  Strategy      *string `json:"strategy"`
  Symbol        *string `json:"symbol"`
  SyncedAt      *string `json:"synced_at"`
  TradeId       *string `json:"trade_id"`
  TradedAt      *string `json:"traded_at"`
  UserId        *string `json:"user_id"`
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

type PublicUserContainersSelect struct {
  CreatedAt  string      `json:"created_at"`
  Mode       string      `json:"mode"`
  Status     string      `json:"status"`
  Strategies interface{} `json:"strategies"`
  UpdatedAt  string      `json:"updated_at"`
  UserId     string      `json:"user_id"`
}

type PublicUserContainersInsert struct {
  CreatedAt  *string     `json:"created_at"`
  Mode       *string     `json:"mode"`
  Status     *string     `json:"status"`
  Strategies interface{} `json:"strategies"`
  UpdatedAt  *string     `json:"updated_at"`
  UserId     string      `json:"user_id"`
}

type PublicUserContainersUpdate struct {
  CreatedAt  *string     `json:"created_at"`
  Mode       *string     `json:"mode"`
  Status     *string     `json:"status"`
  Strategies interface{} `json:"strategies"`
  UpdatedAt  *string     `json:"updated_at"`
  UserId     *string     `json:"user_id"`
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
