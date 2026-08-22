#include <gtest/gtest.h>

#include <unordered_set>

#include "hashable.h"

namespace fixtures
{

TEST(environment_support_test, hashes_types_with_a_hash_member)
{
	std::unordered_set<hashable> values;
	values.emplace(42);
	EXPECT_TRUE(values.contains(hashable(42)));
	EXPECT_FALSE(values.contains(hashable(7)));
}

}
