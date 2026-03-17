package bluetooth

type PHY uint8

const (
	// PHY1M is the 1M PHY, which is the default for Bluetooth LE.
	PHY1M PHY = 0x01
	// PHY2M is the 2M PHY, which allows for higher data rates but consumes more power.
	PHY2M PHY = 0x02
	// PHYCoded is the Coded PHY, which allows for longer range at the cost of lower data rates.
	PHYCoded PHY = 0x03
)
