#include <gtest/gtest.h>

#include "counter.h"

namespace fixtures
{

TEST(counter_increment_test, starts_at_zero)
{
	EXPECT_EQ(counter().value(), 0);
}

TEST(counter_increment_test, increments_once)
{
	counter value;
	value.increment();
	EXPECT_EQ(value.value(), 1);
}

TEST(counter_increment_test, increments_repeatedly)
{
	counter value;
	value.increment();
	value.increment();
	value.increment();
	EXPECT_EQ(value.value(), 3);
}

}
