#pragma once

#include <concepts>
#include <cstddef>
#include <functional>

namespace std
{

// Enables any class exposing a `std::size_t hash()` member as a key in unordered associative containers.
template<typename T>
        requires requires(const T& value) { { value.hash() } -> std::same_as<std::size_t>; }
struct hash<T>
{
	std::size_t operator()(const T& value) const noexcept
	{
		return value.hash();
	}
};

}
