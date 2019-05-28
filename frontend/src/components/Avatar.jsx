import React, { Component } from 'react';
import PropTypes from 'prop-types';
import ReactAvatar from 'react-avatar';

import 'components/Avatar.scss';

class Avatar extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    const { size, username, className } = this.props;
    return (
      <div className={`Avatar ${className}`}>
        <ReactAvatar githubHandle={username} size={size} round={`${size}px`} />
      </div>
    );
  }
}

Avatar.defaultProps = {
  size: '24',
  className: ''
};

Avatar.propTypes = {
  size: PropTypes.string,
  username: PropTypes.string.isRequired,
  className: PropTypes.string
};

export default Avatar;
