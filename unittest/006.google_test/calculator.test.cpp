#include <gtest/gtest.h>

#include <stdexcept>

#include "calculator.h"

namespace fixtures
{

TEST(calculator_test, adds_values)
{
	EXPECT_EQ(calculator().add(20, 22), 42);
}

TEST(calculator_test, subtracts_values)
{
	EXPECT_EQ(calculator().subtract(50, 8), 42);
}

TEST(calculator_test, multiplies_values)
{
	EXPECT_EQ(calculator().multiply(6, 7), 42);
}

TEST(calculator_test, divides_values)
{
	EXPECT_EQ(calculator().divide(84, 2), 42);
}

TEST(calculator_test, rejects_division_by_zero)
{
	EXPECT_THROW(calculator().divide(42, 0), std::invalid_argument);
}

}
