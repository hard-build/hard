#pragma once

#include <memory>

#include "../container/container.h"

namespace fixtures
{

class component
{
public:
	explicit component(int value);

	int value() const;

private:
	int value_;
	std::weak_ptr<container> container_;
};

}
