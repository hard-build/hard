#include "container.h"

void fixtures::container::push(int value)
{
	components_.push_back(std::make_shared<component>(value));
}

std::string fixtures::container::dump() const
{
	std::string result = "[";
	for (std::size_t index = 0; index < components_.size(); ++index)
	{
		if (index != 0)
		{
			result += " ";
		}
		result += std::to_string(components_[index]->value());
	}
	return result + "]";
}
