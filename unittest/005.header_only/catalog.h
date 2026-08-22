#pragma once

#include <cstddef>
#include <vector>

namespace fixtures::header_only
{

template<typename T = int>
class catalog
{
public:
	void add(T value)
	{
		values_.push_back(value);
	}

	std::size_t size() const
	{
		return values_.size();
	}

	T sum() const
	{
		T result{};
		for (const T& value : values_)
		{
			result += value;
		}
		return result;
	}

private:
	std::vector<T> values_;
};

}
