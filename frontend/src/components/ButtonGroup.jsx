import React, { Component } from 'react';
import PropTypes from 'prop-types';

import 'components/ButtonGroup.scss';

class ButtonGroup extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    return (
      <div className="ButtonGroup layout horizontal center">
        {this.props.children && this.props.children}
      </div>
    );
  }
}

ButtonGroup.propTypes = {
  children: PropTypes.node
};

export default ButtonGroup;
