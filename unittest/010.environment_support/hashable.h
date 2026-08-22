#pragma once

#include <cstddef>

namespace fixtures
{

class hashable
{
public:
	explicit hashable(int value) :
	        value_(value)
	{
	}

	std::size_t hash() const noexcept
	{
		return static_cast<std::size_t>(value_);
	}

	bool operator==(const hashable&) const noexcept = default;

private:
	int value_;
};

}
