#!/usr/bin/python3

# This file can be used to recalculate the reward constants in the code when
# changing the reward factor and/or the epoch duration in seconds.

from decimal import Decimal
import decimal

# Number of seconds per epoch.
EPOCH_DURATION_SECONDS = 30

# Total Filecoin supply
SIMPLE_SUPPLY_TOTAL=330000000

# Growth factor per year. Currently 100%.
GROWTH_FACTOR = 1.0

# Seconds in a year, according to filecoin. This is actually slightly shorter
# than a year, but it's close enough.
SECONDS_PER_YEAR=60*60*365*24

# Precision factor.
Q128 = 2**128


# Set the precision to enough digits to store (Q128 * Q128).
#
# This gives us wolfram-alpha level precision.
decimal.getcontext().prec=int(Decimal(Q128**2).log10().to_integral_value(decimal.ROUND_CEILING))

def epochs_in_year() -> Decimal:
    return Decimal(SECONDS_PER_YEAR)/Decimal(EPOCH_DURATION_SECONDS)

def q128(val) -> str:
    return str((Q128 * val).to_integral_value(decimal.ROUND_DOWN))

def atto(val) -> str:
    return str((10**18 * val).to_integral_value(decimal.ROUND_DOWN))

# exp(ln[1 + 200%] / epochsInYear)
def baseline_exponent() -> Decimal: 
    return (Decimal(1 + GROWTH_FACTOR).ln() / epochs_in_year()).exp()

# ln(2) / (6 * epochsInYear)
def reward_lambda() -> Decimal: 
    # 2 is a constant such that the half life is 6 years.
    return Decimal(2).ln() / (6 * epochs_in_year())

# exp(lambda) - 1
def reward_lambda_prime() -> Decimal: 
    return reward_lambda().exp() - 1

# exp(-lambda) - 1
def initial_reward_veolocity_estimate() -> Decimal: 
    return reward_lambda().copy_negate().exp() - 1

def initial_reward_position_estimate() -> Decimal: 
    return (1 - reward_lambda().copy_negate().exp())*SIMPLE_SUPPLY_TOTAL

def main():
    print("BaselineExponent: ", q128(baseline_exponent()))
    print("lambda: ", q128(reward_lambda()))
    print("expLamSubOne: ", q128(reward_lambda_prime()))
    print("InitialRewardVelocityEstimate: ", atto(initial_reward_veolocity_estimate()))
    print("InitialRewardPositionEstimate: ", atto(initial_reward_position_estimate()))

if __name__ == "__main__":
    main()
