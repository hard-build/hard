#include <gtest/gtest.h>

#include "counter.h"

namespace fixtures
{

TEST(counter_reset_test, resets_an_incremented_counter)
{
	counter value;
	value.increment();
	value.reset();
	EXPECT_EQ(value.value(), 0);
}

TEST(counter_reset_test, keeps_zero_at_zero)
{
	counter value;
	value.reset();
	EXPECT_EQ(value.value(), 0);
}

}
