#pragma once

#include <memory>
#include <string>
#include <vector>

#include "../component/component.h"

namespace fixtures
{

class container
{
public:
	void push(int value);

	std::string dump() const;

private:
	std::vector<std::shared_ptr<component>> components_;
};

}
